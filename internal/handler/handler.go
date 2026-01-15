package handler

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/everydev1618/colettedn/internal/generator"
)

type Handler struct {
	gen *generator.Generator
}

func New() *Handler {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	return &Handler{
		gen: generator.New(apiKey),
	}
}

type GenerateRequest struct {
	Keywords []string `json:"keywords"`
	Industry string   `json:"industry"`
	Vibe     string   `json:"vibe"`
	TLDs     []string `json:"tlds"`
}

type GenerateResponse struct {
	Domains []DomainResult `json:"domains"`
	Error   string         `json:"error,omitempty"`
}

type DomainResult struct {
	Name      string `json:"name"`
	Available *bool  `json:"available,omitempty"`
}

func (h *Handler) GenerateDomains(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, GenerateResponse{Error: "Invalid request body"})
		return
	}

	if len(req.Keywords) == 0 {
		writeJSON(w, http.StatusBadRequest, GenerateResponse{Error: "At least one keyword is required"})
		return
	}

	if len(req.TLDs) == 0 {
		req.TLDs = []string{".com", ".io", ".co", ".ai"}
	}

	domains, err := h.gen.Generate(r.Context(), req.Keywords, req.Industry, req.Vibe, req.TLDs)
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
