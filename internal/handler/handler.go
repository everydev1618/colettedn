package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/everydev1618/colettedn/internal/analytics"
	"github.com/everydev1618/colettedn/internal/auth"
	"github.com/everydev1618/colettedn/internal/cache"
	"github.com/everydev1618/colettedn/internal/generator"
	"github.com/everydev1618/colettedn/internal/killswitch"
	"github.com/everydev1618/colettedn/internal/namecheap"
	"github.com/everydev1618/colettedn/internal/ratelimit"
	"github.com/everydev1618/colettedn/internal/user"
)

type Handler struct {
	gen         *generator.Generator
	nc          *namecheap.Client
	cache       cache.Cacher
	limiter     *ratelimit.Limiter
	ks          *killswitch.KillSwitch
	userService user.UserService
}

func New(userService user.UserService) *Handler {
	h := &Handler{
		gen:         generator.New(os.Getenv("ANTHROPIC_API_KEY")),
		userService: userService,
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

	// Initialize kill switch (only in Lambda environment)
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		h.ks = killswitch.New()
	}

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
	Categories       map[string][]DomainResult `json:"categories"`
	Rounds           int                       `json:"rounds"`
	Error            string                    `json:"error,omitempty"`
	UpgradeAvailable bool                      `json:"upgradeAvailable,omitempty"`
	// Usage info for free users
	Usage *UsageInfo `json:"usage,omitempty"`
}

type UsageInfo struct {
	Used      int  `json:"used"`
	Limit     int  `json:"limit"`
	Unlimited bool `json:"unlimited"`
}

const (
	minPerCategory     = 4             // Keep searching until each category has at least this many
	maxRounds          = 5             // Maximum rounds if time allows
	apiGatewayTimeout  = 29 * time.Second // API Gateway hard limit
	timeBufferPerRound = 12 * time.Second // Reserve time for each potential round
)

type DomainResult struct {
	Name      string   `json:"name"`
	Available *bool    `json:"available,omitempty"`
	IsPremium *bool    `json:"isPremium,omitempty"`
	Price     *float64 `json:"price,omitempty"`
	FromCache bool     `json:"fromCache,omitempty"`
	CheckedAt *int64   `json:"checkedAt,omitempty"` // Unix timestamp
	Score     int      `json:"score,omitempty"`     // 0-100 quality score
}

// scoreDomain calculates a quality score (0-100) for a domain name
func scoreDomain(domain string) int {
	// Split into name and TLD
	parts := strings.SplitN(domain, ".", 2)
	if len(parts) != 2 {
		return 50 // Default for invalid format
	}
	name := strings.ToLower(parts[0])
	tld := strings.ToLower(parts[1])

	score := 0

	// Length score (25 points max)
	// Ideal: 5-8 chars, good: 4-10, acceptable: 3-12
	length := len(name)
	switch {
	case length >= 5 && length <= 8:
		score += 25
	case length >= 4 && length <= 10:
		score += 20
	case length >= 3 && length <= 12:
		score += 15
	case length >= 2 && length <= 15:
		score += 10
	default:
		score += 5
	}

	// TLD score (25 points max)
	switch tld {
	case "com":
		score += 25
	case "io", "ai":
		score += 22
	case "co", "app", "dev":
		score += 18
	case "net", "org":
		score += 15
	case "me", "xyz", "tech":
		score += 12
	default:
		score += 10
	}

	// Readability score (25 points max)
	// Based on vowel ratio and letter patterns
	vowels := 0
	for _, c := range name {
		if c == 'a' || c == 'e' || c == 'i' || c == 'o' || c == 'u' {
			vowels++
		}
	}
	vowelRatio := float64(vowels) / float64(len(name))
	switch {
	case vowelRatio >= 0.3 && vowelRatio <= 0.5:
		score += 25 // Ideal ratio
	case vowelRatio >= 0.2 && vowelRatio <= 0.6:
		score += 20
	case vowelRatio >= 0.1:
		score += 12
	default:
		score += 5 // All consonants is hard to pronounce
	}

	// Simplicity score (25 points max)
	// Penalize hyphens, numbers, uncommon patterns
	simplicityScore := 25

	// Check for hyphens
	if strings.Contains(name, "-") {
		simplicityScore -= 10
	}

	// Check for numbers
	for _, c := range name {
		if c >= '0' && c <= '9' {
			simplicityScore -= 8
			break
		}
	}

	// Check for double letters that are hard to type/remember
	hardDoubles := []string{"ii", "uu", "aa", "ww", "yy"}
	for _, d := range hardDoubles {
		if strings.Contains(name, d) {
			simplicityScore -= 3
			break
		}
	}

	// Check for triple consonants (hard to pronounce)
	consonants := "bcdfghjklmnpqrstvwxyz"
	consonantRun := 0
	for _, c := range name {
		if strings.ContainsRune(consonants, c) {
			consonantRun++
			if consonantRun >= 3 {
				simplicityScore -= 5
				break
			}
		} else {
			consonantRun = 0
		}
	}

	if simplicityScore < 0 {
		simplicityScore = 0
	}
	score += simplicityScore

	return score
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

	// Check user subscription tier
	isPro := false
	authUser := auth.GetUser(r.Context())
	if authUser != nil && h.userService != nil {
		fullUser, err := h.userService.GetByID(r.Context(), authUser.UserID)
		if err == nil && fullUser.SubscriptionTier == user.TierPro {
			isPro = true
		}
	}

	// Rate limiting
	ip := getClientIP(r)
	rl := h.limiter.Allow(ip, isPro)
	if !rl.Allowed {
		// Log rate limit violations for monitoring/alerting
		log.Printf("[RATE_LIMIT] ip=%s reason=%s daily_used=%d minute_used=%d isPro=%v",
			ip, rl.Reason, rl.DailyUsed, rl.MinuteUsed, isPro)

		// Track rate limit hit
		analytics.Get().TrackRateLimitHit(r.Context(), ip, string(rl.Reason), isPro)

		// Force refresh kill switch on rate limit violations (faster response to attacks)
		if h.ks != nil {
			h.ks.ForceRefresh()
		}

		w.Header().Set("Retry-After", fmt.Sprintf("%.0f", rl.RetryAfter.Seconds()))
		w.Header().Set("X-RateLimit-Remaining", "0")

		// Include upgrade hint for non-pro users hitting daily limit
		errorMsg := fmt.Sprintf("Rate limit exceeded. Try again in %s.", formatDuration(rl.RetryAfter))
		response := GenerateResponse{Error: errorMsg}
		if rl.Reason == ratelimit.DailyLimit && !isPro {
			response.UpgradeAvailable = true
		}
		writeJSON(w, http.StatusTooManyRequests, response)
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

	// Track search
	var userID, email string
	if authUser != nil {
		userID = authUser.UserID
		email = authUser.Email
	}
	analytics.Get().TrackSearch(r.Context(), userID, email, ip, req.Description, req.TLDStyle)

	// Check if the input is a domain idea (like "tonycto.com") vs a project description
	isDomainMode := isDomainIdea(req.Description)

	// Multi-round generation with availability checking
	availableByCategory := make(map[string][]DomainResult)
	var takenDomains []string
	rounds := 0
	startTime := time.Now()

	for rounds < maxRounds {
		// Check if we have enough time for another round
		elapsed := time.Since(startTime)
		timeRemaining := apiGatewayTimeout - elapsed
		if rounds > 0 && timeRemaining < timeBufferPerRound {
			// Not enough time for another round, return what we have
			log.Printf("[TIMEOUT] Breaking after %d rounds (%.1fs elapsed, need %s buffer)",
				rounds, elapsed.Seconds(), timeBufferPerRound)
			break
		}

		rounds++

		// Generate domains (with exclusions after first round)
		var categorized map[string][]string
		var err error
		if isDomainMode {
			// Domain exploration mode - explore variations of the given domain idea
			if len(takenDomains) == 0 {
				categorized, err = h.gen.GenerateFromDomainIdea(r.Context(), req.Description, tlds)
			} else {
				categorized, err = h.gen.GenerateFromDomainIdeaWithExclusions(r.Context(), req.Description, tlds, takenDomains)
			}
		} else {
			// Normal mode - generate domains based on project description
			if len(takenDomains) == 0 {
				categorized, err = h.gen.GenerateCategorized(r.Context(), req.Description, tlds)
			} else {
				categorized, err = h.gen.GenerateCategorizedWithExclusions(r.Context(), req.Description, tlds, takenDomains)
			}
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
							Score:     scoreDomain(d),
						}
						availableByCategory[cat] = append(availableByCategory[cat], result)
					} else {
						// Taken - add to exclusion list for next round
						takenDomains = append(takenDomains, d)
					}
				} else if h.nc == nil || availabilityErr != nil {
					// No availability checker OR API failed - include as unverified
					result := DomainResult{Name: d, Score: scoreDomain(d)}
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

	// Build response with usage info
	response := GenerateResponse{
		Categories: availableByCategory,
		Rounds:     rounds,
	}

	// Include usage info for tracking
	if isPro {
		response.Usage = &UsageInfo{
			Used:      rl.DailyUsed,
			Limit:     0,
			Unlimited: true,
		}
	} else {
		response.Usage = &UsageInfo{
			Used:      rl.DailyUsed,
			Limit:     h.limiter.DailyLimit(),
			Unlimited: false,
		}
	}

	writeJSON(w, http.StatusOK, response)
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
	requiredCategories := []string{"Professional", "Playful", "Creative", "Minimal"}
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

// Stats returns public statistics for the landing page
type StatsResponse struct {
	TotalSearches int64 `json:"totalSearches"`
	DomainsFound  int64 `json:"domainsFound"`
}

const avgDomainsPerSearch = 18 // ~4 categories × 4-5 domains each

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	totalSearches, err := analytics.Get().GetTotalCount(r.Context(), "searches")
	if err != nil {
		log.Printf("[STATS] Failed to get total searches: %v", err)
		totalSearches = 0
	}

	writeJSON(w, http.StatusOK, StatsResponse{
		TotalSearches: totalSearches,
		DomainsFound:  totalSearches * avgDomainsPerSearch,
	})
}

// TrackAffiliateClick tracks a click to registrar affiliate link
func (h *Handler) TrackAffiliateClick(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain         string `json:"domain"`
		Registrar      string `json:"registrar"`
		OtherRegistrar string `json:"otherRegistrar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	var userID string
	if authUser := auth.GetUser(r.Context()); authUser != nil {
		userID = authUser.UserID
	}

	// If user chose "other", include their specified registrar name
	registrar := req.Registrar
	if req.Registrar == "other" && req.OtherRegistrar != "" {
		registrar = fmt.Sprintf("other:%s", req.OtherRegistrar)
	}

	analytics.Get().TrackAffiliateClick(r.Context(), userID, req.Domain, registrar)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// TrackPageView tracks a page view with referrer
func (h *Handler) TrackPageView(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path     string `json:"path"`
		Referrer string `json:"referrer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	ipAddress := getClientIP(r)
	analytics.Get().TrackPageView(r.Context(), req.Path, req.Referrer, ipAddress)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handler) ServeIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "frontend/index.html")
}

func (h *Handler) ServeWelcomePro(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "frontend/welcome-pro.html")
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

// isDomainIdea checks if the input looks like a domain name idea rather than a project description
func isDomainIdea(input string) bool {
	input = strings.TrimSpace(strings.ToLower(input))

	// Common TLDs to check for
	tlds := []string{".com", ".io", ".co", ".net", ".org", ".ai", ".app", ".dev", ".me", ".xyz", ".tech", ".site", ".online"}

	for _, tld := range tlds {
		if strings.HasSuffix(input, tld) {
			// Make sure there's something before the TLD
			prefix := strings.TrimSuffix(input, tld)
			if len(prefix) > 0 && !strings.Contains(prefix, " ") {
				return true
			}
		}
	}

	return false
}

// ComSiteStatus represents the status of a .com domain's website
type ComSiteStatus string

const (
	ComSiteActive    ComSiteStatus = "active"    // Has a real website
	ComSiteParked    ComSiteStatus = "parked"    // Parked/for sale page
	ComSiteInactive  ComSiteStatus = "inactive"  // No website (error, timeout, etc.)
	ComSiteAvailable ComSiteStatus = "available" // Domain is available for registration
)

type CheckComRequest struct {
	Domain string `json:"domain"` // The non-.com domain to check (e.g., "foo.io")
}

type CheckComResponse struct {
	Domain    string        `json:"domain"`    // The .com domain that was checked
	Status    ComSiteStatus `json:"status"`    // active, parked, inactive, available
	FromCache bool          `json:"fromCache"` // Whether this came from cache
	Error     string        `json:"error,omitempty"`
}

// In-memory cache for .com site checks (TTL: 24 hours)
var (
	comSiteCache   = make(map[string]*comSiteCacheEntry)
	comSiteCacheMu sync.RWMutex
)

type comSiteCacheEntry struct {
	Status    ComSiteStatus
	CheckedAt time.Time
}

const comSiteCacheTTL = 24 * time.Hour

// CheckComSite checks if the .com version of a domain has an active website
func (h *Handler) CheckComSite(w http.ResponseWriter, r *http.Request) {
	var req CheckComRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, CheckComResponse{Error: "Invalid request body"})
		return
	}

	if req.Domain == "" {
		writeJSON(w, http.StatusBadRequest, CheckComResponse{Error: "Domain is required"})
		return
	}

	// Extract base name from domain (e.g., "foo.io" -> "foo")
	domain := strings.TrimSpace(strings.ToLower(req.Domain))
	baseName := extractBaseName(domain)
	if baseName == "" {
		writeJSON(w, http.StatusBadRequest, CheckComResponse{Error: "Invalid domain format"})
		return
	}

	comDomain := baseName + ".com"

	// Check cache first
	comSiteCacheMu.RLock()
	entry, found := comSiteCache[comDomain]
	comSiteCacheMu.RUnlock()

	if found && time.Since(entry.CheckedAt) < comSiteCacheTTL {
		writeJSON(w, http.StatusOK, CheckComResponse{
			Domain:    comDomain,
			Status:    entry.Status,
			FromCache: true,
		})
		return
	}

	// Check if .com is available via Namecheap first
	if h.nc != nil {
		results, err := h.nc.CheckAvailability(r.Context(), []string{comDomain})
		if err == nil && len(results) > 0 && results[0].Available {
			// .com is available for registration
			status := ComSiteAvailable
			cacheComSiteStatus(comDomain, status)
			writeJSON(w, http.StatusOK, CheckComResponse{
				Domain:    comDomain,
				Status:    status,
				FromCache: false,
			})
			return
		}
	}

	// .com is taken - check if it has an active site
	status := checkSiteStatus(r.Context(), comDomain)
	cacheComSiteStatus(comDomain, status)

	writeJSON(w, http.StatusOK, CheckComResponse{
		Domain:    comDomain,
		Status:    status,
		FromCache: false,
	})
}

func extractBaseName(domain string) string {
	// Remove TLD to get base name
	tlds := []string{".com", ".io", ".co", ".net", ".org", ".ai", ".app", ".dev", ".me", ".xyz", ".tech", ".site", ".online"}
	for _, tld := range tlds {
		if strings.HasSuffix(domain, tld) {
			return strings.TrimSuffix(domain, tld)
		}
	}
	// If no known TLD, try to split on last dot
	if idx := strings.LastIndex(domain, "."); idx > 0 {
		return domain[:idx]
	}
	return ""
}

func cacheComSiteStatus(domain string, status ComSiteStatus) {
	comSiteCacheMu.Lock()
	comSiteCache[domain] = &comSiteCacheEntry{
		Status:    status,
		CheckedAt: time.Now(),
	}
	comSiteCacheMu.Unlock()
}

// GenerateTabTitle generates a short title for a search tab
type TabTitleRequest struct {
	SearchPhrase string `json:"searchPhrase"`
}

type TabTitleResponse struct {
	Title string `json:"title"`
	Error string `json:"error,omitempty"`
}

func (h *Handler) GenerateTabTitle(w http.ResponseWriter, r *http.Request) {
	var req TabTitleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, TabTitleResponse{Error: "Invalid request body"})
		return
	}

	if req.SearchPhrase == "" {
		writeJSON(w, http.StatusBadRequest, TabTitleResponse{Error: "Search phrase is required"})
		return
	}

	title, err := h.gen.GenerateTabTitle(r.Context(), req.SearchPhrase)
	if err != nil {
		log.Printf("[TAB_TITLE_ERROR] Failed to generate title: %v", err)
		writeJSON(w, http.StatusInternalServerError, TabTitleResponse{Error: "Failed to generate title"})
		return
	}

	writeJSON(w, http.StatusOK, TabTitleResponse{Title: title})
}

func checkSiteStatus(ctx context.Context, domain string) ComSiteStatus {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 3 redirects
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	// Try HTTPS first, fall back to HTTP
	urls := []string{
		"https://" + domain,
		"http://" + domain,
	}

	for _, url := range urls {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ColetteDN/1.0)")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		// Read limited body for content analysis
		body, err := io.ReadAll(io.LimitReader(resp.Body, 50000)) // 50KB max
		if err != nil {
			continue
		}

		bodyLower := strings.ToLower(string(body))

		// Check for parking page indicators
		parkingIndicators := []string{
			"domain is for sale",
			"buy this domain",
			"this domain may be for sale",
			"domain parking",
			"parked domain",
			"godaddy",
			"sedoparking",
			"hugedomains",
			"dan.com",
			"afternic",
			"undeveloped",
			"domain for sale",
			"make an offer",
			"is available for purchase",
		}

		for _, indicator := range parkingIndicators {
			if strings.Contains(bodyLower, indicator) {
				return ComSiteParked
			}
		}

		// If we got a successful response with real content, it's active
		if resp.StatusCode >= 200 && resp.StatusCode < 400 && len(body) > 500 {
			return ComSiteActive
		}
	}

	return ComSiteInactive
}
