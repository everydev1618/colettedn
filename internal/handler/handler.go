package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/everydev1618/colettedn/internal/cache"
	"github.com/everydev1618/colettedn/internal/generator"
	"github.com/everydev1618/colettedn/internal/namecheap"
)

type Handler struct {
	gen   *generator.Generator
	nc    *namecheap.Client
	cache cache.Cacher
}

func New() *Handler {
	h := &Handler{
		gen: generator.New(os.Getenv("ANTHROPIC_API_KEY")),
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
	Domains []DomainResult `json:"domains"`
	Error   string         `json:"error,omitempty"`
}

type DomainResult struct {
	Name      string   `json:"name"`
	Available *bool    `json:"available,omitempty"`
	IsPremium *bool    `json:"isPremium,omitempty"`
	Price     *float64 `json:"price,omitempty"`
}

type CheckRequest struct {
	Domains []string `json:"domains"`
}

type CheckResponse struct {
	Results []DomainResult `json:"results"`
	Error   string         `json:"error,omitempty"`
}

func (h *Handler) GenerateDomains(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, GenerateResponse{Error: "Invalid request body"})
		return
	}

	if req.Description == "" {
		writeJSON(w, http.StatusBadRequest, GenerateResponse{Error: "Description is required"})
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

	domains, err := h.gen.Generate(r.Context(), req.Description, tlds)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, GenerateResponse{Error: err.Error()})
		return
	}

	results := make([]DomainResult, len(domains))
	for i, d := range domains {
		results[i] = DomainResult{Name: d}
	}

	writeJSON(w, http.StatusOK, GenerateResponse{Domains: results})
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

	// Check cache first
	var uncached []string
	cachedResults := make(map[string]*cache.CachedResult)
	if h.cache != nil {
		cachedResults, uncached = h.cache.GetMany(domains)
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
		} else if api, ok := apiResults[d]; ok {
			results[i].Available = &api.Available
			results[i].IsPremium = &api.IsPremium
			results[i].Price = api.Price
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
