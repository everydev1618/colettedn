package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/everydev1618/colettedn/internal/auth"
	"github.com/everydev1618/colettedn/internal/history"
)

type HistoryHandler struct {
	histService history.HistoryService
}

func NewHistoryHandler() (*HistoryHandler, error) {
	var histService history.HistoryService

	// Use DynamoDB in Lambda, in-memory for local dev
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		var err error
		histService, err = history.NewService("colettedn-history")
		if err != nil {
			return nil, err
		}
	} else {
		// Local development: use in-memory store
		log.Println("[DEV] Using in-memory history store")
		histService = history.NewMemoryService()
	}

	return &HistoryHandler{
		histService: histService,
	}, nil
}

type SaveHistoryRequest struct {
	Description string                          `json:"description"`
	TLDStyle    string                          `json:"tldStyle"`
	Categories  map[string][]history.SearchResult `json:"categories"`
}

type HistoryResponse struct {
	Success bool                   `json:"success"`
	History *history.SearchHistory `json:"history,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

type ListHistoryResponse struct {
	Histories []history.SearchHistory `json:"histories"`
	Error     string                  `json:"error,omitempty"`
}

func (h *HistoryHandler) Save(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, HistoryResponse{Error: "Unauthorized"})
		return
	}

	var req SaveHistoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, HistoryResponse{Error: "Invalid request body"})
		return
	}

	description := strings.TrimSpace(req.Description)
	if description == "" {
		writeJSON(w, http.StatusBadRequest, HistoryResponse{Error: "Description is required"})
		return
	}

	tldStyle := req.TLDStyle
	if tldStyle == "" {
		tldStyle = "traditional"
	}

	hist, err := h.histService.Save(r.Context(), user.UserID, description, tldStyle, req.Categories)
	if err != nil {
		log.Printf("Failed to save history: %v", err)
		writeJSON(w, http.StatusInternalServerError, HistoryResponse{Error: "Failed to save history"})
		return
	}

	writeJSON(w, http.StatusOK, HistoryResponse{Success: true, History: hist})
}

func (h *HistoryHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, ListHistoryResponse{Error: "Unauthorized"})
		return
	}

	// Get optional limit from query params
	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	histories, err := h.histService.List(r.Context(), user.UserID, limit)
	if err != nil {
		log.Printf("Failed to list history: %v", err)
		writeJSON(w, http.StatusInternalServerError, ListHistoryResponse{Error: "Failed to get history"})
		return
	}

	if histories == nil {
		histories = []history.SearchHistory{}
	}

	writeJSON(w, http.StatusOK, ListHistoryResponse{Histories: histories})
}

func (h *HistoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, HistoryResponse{Error: "Unauthorized"})
		return
	}

	// Extract searchedAt from path: /api/history/{searchedAt}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		writeJSON(w, http.StatusBadRequest, HistoryResponse{Error: "Invalid path"})
		return
	}
	searchedAtStr := parts[len(parts)-1]

	searchedAt, err := strconv.ParseInt(searchedAtStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, HistoryResponse{Error: "Invalid timestamp"})
		return
	}

	if err := h.histService.Delete(r.Context(), user.UserID, searchedAt); err != nil {
		log.Printf("Failed to delete history: %v", err)
		writeJSON(w, http.StatusInternalServerError, HistoryResponse{Error: "Failed to delete history"})
		return
	}

	writeJSON(w, http.StatusOK, HistoryResponse{Success: true})
}

func (h *HistoryHandler) GetService() history.HistoryService {
	return h.histService
}
