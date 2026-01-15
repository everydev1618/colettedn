package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/everydev1618/colettedn/internal/auth"
	"github.com/everydev1618/colettedn/internal/owned"
)

type OwnedHandler struct {
	ownedService owned.OwnedService
}

func NewOwnedHandler() (*OwnedHandler, error) {
	var ownedService owned.OwnedService

	// Use DynamoDB in Lambda, in-memory for local dev
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		var err error
		ownedService, err = owned.NewService("colettedn-owned")
		if err != nil {
			return nil, err
		}
	} else {
		// Local development: use in-memory store
		log.Println("[DEV] Using in-memory owned domains store")
		ownedService = owned.NewMemoryService()
	}

	return &OwnedHandler{
		ownedService: ownedService,
	}, nil
}

type AddOwnedRequest struct {
	Domain          string `json:"domain"`
	AcquisitionType string `json:"acquisitionType"`
}

type OwnedResponse struct {
	Success bool               `json:"success"`
	Owned   *owned.OwnedDomain `json:"owned,omitempty"`
	Error   string             `json:"error,omitempty"`
}

type ListOwnedResponse struct {
	Owned []owned.OwnedDomain `json:"owned"`
	Error string              `json:"error,omitempty"`
}

func (h *OwnedHandler) Add(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, OwnedResponse{Error: "Unauthorized"})
		return
	}

	var req AddOwnedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, OwnedResponse{Error: "Invalid request body"})
		return
	}

	domain := strings.TrimSpace(strings.ToLower(req.Domain))
	if domain == "" {
		writeJSON(w, http.StatusBadRequest, OwnedResponse{Error: "Domain is required"})
		return
	}

	// Validate acquisition type
	var acquisitionType owned.AcquisitionType
	switch req.AcquisitionType {
	case "previously_owned":
		acquisitionType = owned.AcquisitionPreviouslyOwned
	case "found_via_colette":
		acquisitionType = owned.AcquisitionFoundViaColette
	default:
		writeJSON(w, http.StatusBadRequest, OwnedResponse{Error: "Invalid acquisition type"})
		return
	}

	ownedDomain, err := h.ownedService.Add(r.Context(), user.UserID, domain, acquisitionType)
	if err != nil {
		log.Printf("[OWNED_ERROR] Failed to add owned domain: %v", err)
		writeJSON(w, http.StatusInternalServerError, OwnedResponse{Error: "Failed to add owned domain"})
		return
	}

	writeJSON(w, http.StatusOK, OwnedResponse{Success: true, Owned: ownedDomain})
}

func (h *OwnedHandler) Remove(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, OwnedResponse{Error: "Unauthorized"})
		return
	}

	// Extract domain from path: /api/owned/{domain}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		writeJSON(w, http.StatusBadRequest, OwnedResponse{Error: "Domain is required"})
		return
	}
	domain := strings.TrimSpace(strings.ToLower(parts[len(parts)-1]))

	if domain == "" {
		writeJSON(w, http.StatusBadRequest, OwnedResponse{Error: "Domain is required"})
		return
	}

	if err := h.ownedService.Remove(r.Context(), user.UserID, domain); err != nil {
		log.Printf("[OWNED_ERROR] Failed to remove owned domain: %v", err)
		writeJSON(w, http.StatusInternalServerError, OwnedResponse{Error: "Failed to remove owned domain"})
		return
	}

	writeJSON(w, http.StatusOK, OwnedResponse{Success: true})
}

func (h *OwnedHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, ListOwnedResponse{Error: "Unauthorized"})
		return
	}

	domains, err := h.ownedService.List(r.Context(), user.UserID)
	if err != nil {
		log.Printf("[OWNED_ERROR] Failed to list owned domains: %v", err)
		writeJSON(w, http.StatusInternalServerError, ListOwnedResponse{Error: "Failed to get owned domains"})
		return
	}

	if domains == nil {
		domains = []owned.OwnedDomain{}
	}

	writeJSON(w, http.StatusOK, ListOwnedResponse{Owned: domains})
}

func (h *OwnedHandler) GetService() owned.OwnedService {
	return h.ownedService
}
