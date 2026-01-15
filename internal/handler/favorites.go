package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/everydev1618/colettedn/internal/auth"
	"github.com/everydev1618/colettedn/internal/favorites"
)

type FavoritesHandler struct {
	favService favorites.FavoritesService
}

func NewFavoritesHandler() (*FavoritesHandler, error) {
	var favService favorites.FavoritesService

	// Use DynamoDB in Lambda, in-memory for local dev
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		var err error
		favService, err = favorites.NewService("colettedn-favorites")
		if err != nil {
			return nil, err
		}
	} else {
		// Local development: use in-memory store
		log.Println("[DEV] Using in-memory favorites store")
		favService = favorites.NewMemoryService()
	}

	return &FavoritesHandler{
		favService: favService,
	}, nil
}

type AddFavoriteRequest struct {
	Domain string `json:"domain"`
}

type FavoriteResponse struct {
	Success  bool                 `json:"success"`
	Favorite *favorites.Favorite  `json:"favorite,omitempty"`
	Error    string               `json:"error,omitempty"`
}

type ListFavoritesResponse struct {
	Favorites []favorites.Favorite `json:"favorites"`
	Error     string               `json:"error,omitempty"`
}

func (h *FavoritesHandler) Add(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, FavoriteResponse{Error: "Unauthorized"})
		return
	}

	var req AddFavoriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, FavoriteResponse{Error: "Invalid request body"})
		return
	}

	domain := strings.TrimSpace(strings.ToLower(req.Domain))
	if domain == "" {
		writeJSON(w, http.StatusBadRequest, FavoriteResponse{Error: "Domain is required"})
		return
	}

	fav, err := h.favService.Add(r.Context(), user.UserID, domain)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, FavoriteResponse{Error: "Failed to add favorite"})
		return
	}

	writeJSON(w, http.StatusOK, FavoriteResponse{Success: true, Favorite: fav})
}

func (h *FavoritesHandler) Remove(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, FavoriteResponse{Error: "Unauthorized"})
		return
	}

	// Extract domain from path: /api/favorites/{domain}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		writeJSON(w, http.StatusBadRequest, FavoriteResponse{Error: "Domain is required"})
		return
	}
	domain := strings.TrimSpace(strings.ToLower(parts[len(parts)-1]))

	if domain == "" {
		writeJSON(w, http.StatusBadRequest, FavoriteResponse{Error: "Domain is required"})
		return
	}

	if err := h.favService.Remove(r.Context(), user.UserID, domain); err != nil {
		writeJSON(w, http.StatusInternalServerError, FavoriteResponse{Error: "Failed to remove favorite"})
		return
	}

	writeJSON(w, http.StatusOK, FavoriteResponse{Success: true})
}

func (h *FavoritesHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, ListFavoritesResponse{Error: "Unauthorized"})
		return
	}

	favs, err := h.favService.List(r.Context(), user.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ListFavoritesResponse{Error: "Failed to get favorites"})
		return
	}

	if favs == nil {
		favs = []favorites.Favorite{}
	}

	writeJSON(w, http.StatusOK, ListFavoritesResponse{Favorites: favs})
}

func (h *FavoritesHandler) GetService() favorites.FavoritesService {
	return h.favService
}
