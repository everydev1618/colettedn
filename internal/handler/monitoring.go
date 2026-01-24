package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/everydev1618/colettedn/internal/auth"
	"github.com/everydev1618/colettedn/internal/monitoring"
	"github.com/everydev1618/colettedn/internal/rdap"
	"github.com/everydev1618/colettedn/internal/user"
)

type MonitoringHandler struct {
	monitoringService monitoring.MonitoringService
	userService       user.UserService
	rdap              *rdap.Client
}

func NewMonitoringHandler(userService user.UserService) (*MonitoringHandler, error) {
	var monitoringService monitoring.MonitoringService

	// Use DynamoDB in Lambda, in-memory for local dev
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		var err error
		monitoringService, err = monitoring.NewService("colettedn-monitoring")
		if err != nil {
			return nil, err
		}
	} else {
		// Local development: use in-memory store
		log.Println("[DEV] Using in-memory monitoring store")
		monitoringService = monitoring.NewMemoryService()
	}

	return &MonitoringHandler{
		monitoringService: monitoringService,
		userService:       userService,
		rdap:              rdap.New(),
	}, nil
}

type AddMonitoringRequest struct {
	Domain          string  `json:"domain"`
	ExpirationDate  *string `json:"expirationDate,omitempty"`
	DaysUntilExpiry *int    `json:"daysUntilExpiry,omitempty"`
	Registrar       string  `json:"registrar,omitempty"`
}

type MonitoringResponse struct {
	Success    bool                        `json:"success"`
	Monitoring *monitoring.MonitoredDomain `json:"monitoring,omitempty"`
	Error      string                      `json:"error,omitempty"`
}

type ListMonitoringResponse struct {
	Monitoring []monitoring.MonitoredDomain `json:"monitoring"`
	Error      string                       `json:"error,omitempty"`
}

// isPro checks if the current user has a Pro subscription
func (h *MonitoringHandler) isPro(r *http.Request) bool {
	authUser := auth.GetUser(r.Context())
	if authUser == nil || h.userService == nil {
		return false
	}
	fullUser, err := h.userService.GetByID(r.Context(), authUser.UserID)
	if err != nil {
		return false
	}
	return fullUser.SubscriptionTier == user.TierPro
}

func (h *MonitoringHandler) Add(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetUser(r.Context())
	if authUser == nil {
		writeJSON(w, http.StatusUnauthorized, MonitoringResponse{Error: "Unauthorized"})
		return
	}

	// Require PRO subscription
	if !h.isPro(r) {
		writeJSON(w, http.StatusForbidden, MonitoringResponse{Error: "Pro subscription required"})
		return
	}

	var req AddMonitoringRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, MonitoringResponse{Error: "Invalid request body"})
		return
	}

	domain := strings.TrimSpace(strings.ToLower(req.Domain))
	if domain == "" {
		writeJSON(w, http.StatusBadRequest, MonitoringResponse{Error: "Domain is required"})
		return
	}

	monitored := &monitoring.MonitoredDomain{
		Domain:          domain,
		ExpirationDate:  req.ExpirationDate,
		DaysUntilExpiry: req.DaysUntilExpiry,
		Registrar:       req.Registrar,
	}

	result, err := h.monitoringService.Add(r.Context(), authUser.UserID, monitored)
	if err != nil {
		log.Printf("[MONITORING_ERROR] Failed to add monitored domain: %v", err)
		writeJSON(w, http.StatusInternalServerError, MonitoringResponse{Error: "Failed to add monitored domain"})
		return
	}

	writeJSON(w, http.StatusOK, MonitoringResponse{Success: true, Monitoring: result})
}

func (h *MonitoringHandler) Remove(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetUser(r.Context())
	if authUser == nil {
		writeJSON(w, http.StatusUnauthorized, MonitoringResponse{Error: "Unauthorized"})
		return
	}

	// Require PRO subscription
	if !h.isPro(r) {
		writeJSON(w, http.StatusForbidden, MonitoringResponse{Error: "Pro subscription required"})
		return
	}

	// Extract domain from path: /api/monitoring/{domain}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		writeJSON(w, http.StatusBadRequest, MonitoringResponse{Error: "Domain is required"})
		return
	}
	domain := strings.TrimSpace(strings.ToLower(parts[len(parts)-1]))

	if domain == "" {
		writeJSON(w, http.StatusBadRequest, MonitoringResponse{Error: "Domain is required"})
		return
	}

	if err := h.monitoringService.Remove(r.Context(), authUser.UserID, domain); err != nil {
		log.Printf("[MONITORING_ERROR] Failed to remove monitored domain: %v", err)
		writeJSON(w, http.StatusInternalServerError, MonitoringResponse{Error: "Failed to remove monitored domain"})
		return
	}

	writeJSON(w, http.StatusOK, MonitoringResponse{Success: true})
}

func (h *MonitoringHandler) List(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetUser(r.Context())
	if authUser == nil {
		writeJSON(w, http.StatusUnauthorized, ListMonitoringResponse{Error: "Unauthorized"})
		return
	}

	// Require PRO subscription
	if !h.isPro(r) {
		writeJSON(w, http.StatusForbidden, ListMonitoringResponse{Error: "Pro subscription required"})
		return
	}

	domains, err := h.monitoringService.List(r.Context(), authUser.UserID)
	if err != nil {
		log.Printf("[MONITORING_ERROR] Failed to list monitored domains: %v", err)
		writeJSON(w, http.StatusInternalServerError, ListMonitoringResponse{Error: "Failed to get monitored domains"})
		return
	}

	if domains == nil {
		domains = []monitoring.MonitoredDomain{}
	}

	writeJSON(w, http.StatusOK, ListMonitoringResponse{Monitoring: domains})
}

func (h *MonitoringHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetUser(r.Context())
	if authUser == nil {
		writeJSON(w, http.StatusUnauthorized, MonitoringResponse{Error: "Unauthorized"})
		return
	}

	// Require PRO subscription
	if !h.isPro(r) {
		writeJSON(w, http.StatusForbidden, MonitoringResponse{Error: "Pro subscription required"})
		return
	}

	// Extract domain from path: /api/monitoring/{domain}/refresh
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		writeJSON(w, http.StatusBadRequest, MonitoringResponse{Error: "Domain is required"})
		return
	}
	domain := strings.TrimSpace(strings.ToLower(parts[len(parts)-2]))

	if domain == "" {
		writeJSON(w, http.StatusBadRequest, MonitoringResponse{Error: "Domain is required"})
		return
	}

	// Get existing monitored domain
	existing, err := h.monitoringService.Get(r.Context(), authUser.UserID, domain)
	if err != nil {
		log.Printf("[MONITORING_ERROR] Failed to get monitored domain: %v", err)
		writeJSON(w, http.StatusInternalServerError, MonitoringResponse{Error: "Failed to get monitored domain"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, MonitoringResponse{Error: "Domain not found in monitoring list"})
		return
	}

	// Fetch fresh RDAP data
	rdapInfo, err := h.rdap.Lookup(r.Context(), domain)
	if err != nil {
		log.Printf("[MONITORING_ERROR] RDAP lookup failed for %s: %v", domain, err)
		writeJSON(w, http.StatusInternalServerError, MonitoringResponse{Error: "Failed to fetch domain info"})
		return
	}

	if rdapInfo.Error != "" {
		log.Printf("[MONITORING_WARN] RDAP error for %s: %s", domain, rdapInfo.Error)
	}

	// Update with fresh data
	if rdapInfo.ExpirationDate != nil {
		expStr := rdapInfo.ExpirationDate.Format("2006-01-02")
		existing.ExpirationDate = &expStr
	}
	existing.DaysUntilExpiry = rdapInfo.DaysUntilExpiry
	existing.Registrar = rdapInfo.Registrar

	if err := h.monitoringService.Update(r.Context(), authUser.UserID, existing); err != nil {
		log.Printf("[MONITORING_ERROR] Failed to update monitored domain: %v", err)
		writeJSON(w, http.StatusInternalServerError, MonitoringResponse{Error: "Failed to update monitored domain"})
		return
	}

	writeJSON(w, http.StatusOK, MonitoringResponse{Success: true, Monitoring: existing})
}

func (h *MonitoringHandler) GetService() monitoring.MonitoringService {
	return h.monitoringService
}
