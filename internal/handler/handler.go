package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/everydev1618/colettedn/internal/cache"
	"github.com/everydev1618/colettedn/internal/generator"
	"github.com/everydev1618/colettedn/internal/killswitch"
	"github.com/everydev1618/colettedn/internal/namecheap"
	"github.com/everydev1618/colettedn/internal/ratelimit"
)

type Handler struct {
	gen     *generator.Generator
	nc      *namecheap.Client
	cache   cache.Cacher
	limiter *ratelimit.Limiter
	ks      *killswitch.KillSwitch
}

func New() *Handler {
	h := &Handler{
		gen: generator.New(os.Getenv("ANTHROPIC_API_KEY")),
	}

	// Initialize rate limiter (configurable via env vars)
	perMinute := 5
	dailyLimit := 30
	if v := os.Getenv("RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			perMinute = n
		}
	}
	if v := os.Getenv("RATE_LIMIT_DAILY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			dailyLimit = n
		}
	}
	h.limiter = ratelimit.New(ratelimit.Config{
		PerMinute:  perMinute,
		DailyLimit: dailyLimit,
	})

	// Kill switch temporarily disabled - re-enable after IAM propagates
	// if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
	// 	h.ks = killswitch.New()
	// }

	// Initialize Namecheap client if configured
	if os.Getenv("NAMECHEAP_API_KEY") != "" {
		h.nc = namecheap.New(namecheap.Config{
			APIUser:  os.Getenv("NAMECHEAP_API_USER"),
			APIKey:   os.Getenv("NAMECHEAP_API_KEY"),
			Username: os.Getenv("NAMECHEAP_USERNAME"),
			ClientIP: os.Getenv("NAMECHEAP_CLIENT_IP"),
			Sandbox:  os.Getenv("NAMECHEAP_SANDBOX") == "true",
		})
	}

	// Initialize cache (24 hour TTL for availability results)
	// Use DynamoDB in Lambda, SQLite locally
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		c, err := cache.NewDynamo("colettedn-cache", 24*time.Hour)
		if err == nil {
			h.cache = c
		}
	} else {
		dbPath := os.Getenv("CACHE_DB_PATH")
		if dbPath == "" {
			dbPath = "cache.db"
		}
		c, err := cache.NewSQLite(dbPath, 24*time.Hour)
		if err == nil {
			h.cache = c
		}
	}

	return h
}

type GenerateRequest struct {
	Description string `json:"description"`
	TLDStyle    string `json:"tldStyle"`
}

type GenerateResponse struct {
	Categories map[string][]DomainResult `json:"categories"`
	Rounds     int                       `json:"rounds"`
	Error      string                    `json:"error,omitempty"`
}

const (
	minPerCategory = 4 // Keep searching until each category has at least this many
	maxRounds      = 2 // Reduced from 5 to stay under API Gateway's 29s timeout
)

type DomainResult struct {
	Name      string   `json:"name"`
	Available *bool    `json:"available,omitempty"`
	IsPremium *bool    `json:"isPremium,omitempty"`
	Price     *float64 `json:"price,omitempty"`
	FromCache bool     `json:"fromCache,omitempty"`
	CheckedAt *int64   `json:"checkedAt,omitempty"` // Unix timestamp
}

type CheckRequest struct {
	Domains      []string `json:"domains"`
	ForceRefresh []string `json:"forceRefresh,omitempty"` // Domains to bypass cache for
}

type CheckResponse struct {
	Results []DomainResult `json:"results"`
	Error   string         `json:"error,omitempty"`
}

func (h *Handler) GenerateDomains(w http.ResponseWriter, r *http.Request) {
	// Kill switch check
	if h.ks != nil && h.ks.IsDisabled() {
		writeJSON(w, http.StatusServiceUnavailable, GenerateResponse{
			Error: "Service temporarily unavailable. Please try again later.",
		})
		return
	}

	// Rate limiting
	ip := getClientIP(r)
	rl := h.limiter.Allow(ip)
	if !rl.Allowed {
		// Log rate limit violations for monitoring/alerting
		log.Printf("[RATE_LIMIT] ip=%s reason=%s daily_used=%d minute_used=%d",
			ip, rl.Reason, rl.DailyUsed, rl.MinuteUsed)

		// Force refresh kill switch on rate limit violations (faster response to attacks)
		if h.ks != nil {
			h.ks.ForceRefresh()
		}

		w.Header().Set("Retry-After", fmt.Sprintf("%.0f", rl.RetryAfter.Seconds()))
		w.Header().Set("X-RateLimit-Remaining", "0")
		writeJSON(w, http.StatusTooManyRequests, GenerateResponse{
			Error: fmt.Sprintf("Rate limit exceeded. Try again in %s.", formatDuration(rl.RetryAfter)),
		})
		return
	}
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", rl.DailyRemaining))

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, GenerateResponse{Error: "Invalid request body"})
		return
	}

	if req.Description == "" {
		writeJSON(w, http.StatusBadRequest, GenerateResponse{Error: "Description is required"})
		return
	}

	if len(req.Description) > 500 {
		writeJSON(w, http.StatusBadRequest, GenerateResponse{Error: "Description too long (max 500 characters)"})
		return
	}

	// Convert TLD style to actual TLDs
	var tlds []string
	if req.TLDStyle == "creative" {
		tlds = []string{".com", ".io", ".ai", ".app", ".dev", ".co"}
	} else {
		// Traditional (default)
		tlds = []string{".com", ".co", ".net", ".org"}
	}

	// Multi-round generation with availability checking
	availableByCategory := make(map[string][]DomainResult)
	var takenDomains []string
	rounds := 0

	for rounds < maxRounds {
		rounds++

		// Generate domains (with exclusions after first round)
		var categorized map[string][]string
		var err error
		if len(takenDomains) == 0 {
			categorized, err = h.gen.GenerateCategorized(r.Context(), req.Description, tlds)
		} else {
			categorized, err = h.gen.GenerateCategorizedWithExclusions(r.Context(), req.Description, tlds, takenDomains)
		}
		if err != nil {
			// If we have some results, return those; otherwise error
			if countAvailable(availableByCategory) > 0 {
				break
			}
			writeJSON(w, http.StatusInternalServerError, GenerateResponse{Error: err.Error()})
			return
		}

		// Collect all new domains for availability check
		var newDomains []string
		for _, domains := range categorized {
			newDomains = append(newDomains, domains...)
		}

		// Check availability (if Namecheap is configured)
		availabilityMap := make(map[string]*availabilityInfo)
		var availabilityErr error
		if h.nc != nil && len(newDomains) > 0 {
			availabilityMap, availabilityErr = h.checkDomainsAvailability(r, newDomains)
		}

		// Sort domains into available/taken
		for cat, domains := range categorized {
			for _, d := range domains {
				d = strings.ToLower(d)

				if info, ok := availabilityMap[d]; ok {
					if info.available {
						// Available - add to results
						result := DomainResult{
							Name:      d,
							Available: &info.available,
							IsPremium: &info.isPremium,
							Price:     info.price,
							FromCache: info.fromCache,
							CheckedAt: info.checkedAt,
						}
						availableByCategory[cat] = append(availableByCategory[cat], result)
					} else {
						// Taken - add to exclusion list for next round
						takenDomains = append(takenDomains, d)
					}
				} else if h.nc == nil || availabilityErr != nil {
					// No availability checker OR API failed - include as unverified
					result := DomainResult{Name: d}
					availableByCategory[cat] = append(availableByCategory[cat], result)
				}
			}
		}

		// Check if we have enough available domains in each category
		// Also stop if Namecheap API is failing (no point in multiple rounds)
		if hasEnoughPerCategory(availableByCategory) || h.nc == nil || availabilityErr != nil {
			break
		}
	}

	writeJSON(w, http.StatusOK, GenerateResponse{
		Categories: availableByCategory,
		Rounds:     rounds,
	})
}

type availabilityInfo struct {
	available bool
	isPremium bool
	price     *float64
	fromCache bool
	checkedAt *int64
}

func (h *Handler) checkDomainsAvailability(r *http.Request, domains []string) (map[string]*availabilityInfo, error) {
	result := make(map[string]*availabilityInfo)

	// Check cache first
	var uncached []string
	if h.cache != nil {
		cached, notCached := h.cache.GetMany(domains)
		for d, c := range cached {
			checkedAt := c.CheckedAt.Unix()
			result[d] = &availabilityInfo{
				available: c.Available,
				isPremium: c.IsPremium,
				price:     c.Price,
				fromCache: true,
				checkedAt: &checkedAt,
			}
		}
		uncached = notCached
	} else {
		uncached = domains
	}

	// Fetch uncached from API
	if len(uncached) > 0 {
		apiResults, err := h.nc.CheckAvailability(r.Context(), uncached)
		if err != nil {
			log.Printf("[NAMECHEAP_ERROR] Failed to check %d domains: %v", len(uncached), err)
			return result, err
		}
		now := time.Now().Unix()
		for _, ar := range apiResults {
			key := strings.ToLower(ar.Domain)
			result[key] = &availabilityInfo{
				available: ar.Available,
				isPremium: ar.IsPremium,
				price:     ar.Price,
				fromCache: false,
				checkedAt: &now,
			}
			// Cache the result
			if h.cache != nil {
				h.cache.Set(key, ar.Available, ar.IsPremium, ar.Price)
			}
		}
	}

	return result, nil
}

func countAvailable(categories map[string][]DomainResult) int {
	total := 0
	for _, results := range categories {
		total += len(results)
	}
	return total
}

func hasEnoughPerCategory(categories map[string][]DomainResult) bool {
	requiredCategories := []string{"Professional", "Playful", "Techy", "Minimal"}
	for _, cat := range requiredCategories {
		if len(categories[cat]) < minPerCategory {
			return false
		}
	}
	return true
}

func (h *Handler) CheckAvailability(w http.ResponseWriter, r *http.Request) {
	if h.nc == nil {
		writeJSON(w, http.StatusServiceUnavailable, CheckResponse{Error: "Availability checking not configured"})
		return
	}

	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, CheckResponse{Error: "Invalid request body"})
		return
	}

	if len(req.Domains) == 0 {
		writeJSON(w, http.StatusBadRequest, CheckResponse{Error: "No domains provided"})
		return
	}

	// Clean domain names
	var domains []string
	for _, d := range req.Domains {
		d = strings.TrimSpace(strings.ToLower(d))
		if d != "" {
			domains = append(domains, d)
		}
	}

	// Build set of domains to force refresh
	forceRefreshSet := make(map[string]bool)
	for _, d := range req.ForceRefresh {
		forceRefreshSet[strings.TrimSpace(strings.ToLower(d))] = true
	}

	// Check cache first (excluding force refresh domains)
	var uncached []string
	cachedResults := make(map[string]*cache.CachedResult)
	if h.cache != nil {
		// Split domains into cacheable and force-refresh
		var cacheableDomains []string
		for _, d := range domains {
			if forceRefreshSet[d] {
				uncached = append(uncached, d)
			} else {
				cacheableDomains = append(cacheableDomains, d)
			}
		}
		cached, notCached := h.cache.GetMany(cacheableDomains)
		cachedResults = cached
		uncached = append(uncached, notCached...)
	} else {
		uncached = domains
	}

	// Fetch uncached from API
	apiResults := make(map[string]namecheap.DomainStatus)
	if len(uncached) > 0 {
		results, err := h.nc.CheckAvailability(r.Context(), uncached)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, CheckResponse{Error: err.Error()})
			return
		}

		for _, r := range results {
			key := strings.ToLower(r.Domain)
			apiResults[key] = r
			// Cache the result
			if h.cache != nil {
				h.cache.Set(key, r.Available, r.IsPremium, r.Price)
			}
		}
	}

	// Build response
	results := make([]DomainResult, len(domains))
	for i, d := range domains {
		results[i] = DomainResult{Name: d}

		if cached, ok := cachedResults[d]; ok {
			results[i].Available = &cached.Available
			results[i].IsPremium = &cached.IsPremium
			results[i].Price = cached.Price
			results[i].FromCache = true
			checkedAt := cached.CheckedAt.Unix()
			results[i].CheckedAt = &checkedAt
		} else if api, ok := apiResults[d]; ok {
			results[i].Available = &api.Available
			results[i].IsPremium = &api.IsPremium
			results[i].Price = api.Price
			// Fresh result, not from cache
			now := time.Now().Unix()
			results[i].CheckedAt = &now
		}
	}

	writeJSON(w, http.StatusOK, CheckResponse{Results: results})
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ServeIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "frontend/index.html")
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// getClientIP extracts the client IP from the request, handling proxies
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For (common for proxies/load balancers)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP (nginx)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	return fmt.Sprintf("%.1f hours", d.Hours())
}
