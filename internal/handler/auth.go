package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"

	"github.com/everydev1618/colettedn/internal/auth"
	"github.com/everydev1618/colettedn/internal/user"
)

type AuthHandler struct {
	authService auth.TokenService
	userService user.UserService
	emailSender *auth.EmailSender
	appURL      string
}

func NewAuthHandler() (*AuthHandler, error) {
	var authService auth.TokenService
	var userService user.UserService

	// Use DynamoDB in Lambda, in-memory for local dev
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		var err error
		authService, err = auth.NewService("colettedn-tokens")
		if err != nil {
			return nil, err
		}

		userService, err = user.NewService("colettedn-users")
		if err != nil {
			return nil, err
		}
	} else {
		// Local development: use in-memory stores
		log.Println("[DEV] Using in-memory auth and user stores")
		authService = auth.NewMemoryService()
		userService = user.NewMemoryService()
	}

	fromEmail := os.Getenv("FROM_EMAIL")
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:8080"
	}

	var emailSender *auth.EmailSender
	if fromEmail != "" && os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		var err error
		emailSender, err = auth.NewEmailSender(fromEmail, appURL)
		if err != nil {
			log.Printf("[WARN] Failed to initialize email sender: %v", err)
		}
	}

	return &AuthHandler{
		authService: authService,
		userService: userService,
		emailSender: emailSender,
		appURL:      appURL,
	}, nil
}

type LoginRequest struct {
	Email string `json:"email"`
}

type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, LoginResponse{Error: "Invalid request body"})
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		writeJSON(w, http.StatusBadRequest, LoginResponse{Error: "Email is required"})
		return
	}

	// Validate email format
	if _, err := mail.ParseAddress(email); err != nil {
		writeJSON(w, http.StatusBadRequest, LoginResponse{Error: "Invalid email format"})
		return
	}

	// Create magic link token
	token, err := h.authService.CreateMagicLinkToken(r.Context(), email)
	if err != nil {
		log.Printf("[AUTH_ERROR] Failed to create magic link: %v", err)
		writeJSON(w, http.StatusInternalServerError, LoginResponse{Error: "Failed to send login email"})
		return
	}

	// Send email
	if h.emailSender != nil {
		if err := h.emailSender.SendMagicLink(r.Context(), email, token); err != nil {
			log.Printf("[EMAIL_ERROR] Failed to send magic link email: %v", err)
			writeJSON(w, http.StatusInternalServerError, LoginResponse{Error: "Failed to send login email"})
			return
		}
	} else {
		// Local dev: log the magic link
		log.Printf("[DEV] Magic link for %s: %s/api/auth/verify?token=%s", email, h.appURL, token)
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		Success: true,
		Message: "Check your email for a login link",
	})
}

type VerifyResponse struct {
	Success bool       `json:"success"`
	Token   string     `json:"token,omitempty"`
	User    *user.User `json:"user,omitempty"`
	Error   string     `json:"error,omitempty"`
}

func (h *AuthHandler) Verify(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		writeJSON(w, http.StatusBadRequest, VerifyResponse{Error: "Token is required"})
		return
	}

	// Verify the magic link token
	magicToken, err := h.authService.VerifyMagicLinkToken(r.Context(), tokenStr)
	if err != nil {
		if err == auth.ErrTokenNotFound || err == auth.ErrTokenExpired {
			writeJSON(w, http.StatusBadRequest, VerifyResponse{Error: "Invalid or expired link"})
			return
		}
		log.Printf("[AUTH_ERROR] Failed to verify token: %v", err)
		writeJSON(w, http.StatusInternalServerError, VerifyResponse{Error: "Verification failed"})
		return
	}

	// Get or create user
	u, err := h.userService.GetOrCreate(r.Context(), magicToken.Email)
	if err != nil {
		log.Printf("[USER_ERROR] Failed to get/create user: %v", err)
		writeJSON(w, http.StatusInternalServerError, VerifyResponse{Error: "Failed to create account"})
		return
	}

	// Create session token
	sessionToken, err := h.authService.CreateSessionToken(r.Context(), u.UserID, u.Email)
	if err != nil {
		log.Printf("[AUTH_ERROR] Failed to create session: %v", err)
		writeJSON(w, http.StatusInternalServerError, VerifyResponse{Error: "Failed to create session"})
		return
	}

	// For browser redirect (when clicking email link), redirect with token in fragment
	if r.Header.Get("Accept") == "" || strings.Contains(r.Header.Get("Accept"), "text/html") {
		redirectURL := h.appURL + "/#token=" + sessionToken
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	writeJSON(w, http.StatusOK, VerifyResponse{
		Success: true,
		Token:   sessionToken,
		User:    u,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			_ = h.authService.DeleteSessionToken(r.Context(), parts[1])
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

type MeResponse struct {
	User  *user.User `json:"user,omitempty"`
	Error string     `json:"error,omitempty"`
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	u := auth.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, MeResponse{Error: "Unauthorized"})
		return
	}

	fullUser, err := h.userService.GetByID(r.Context(), u.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, MeResponse{Error: "Failed to get user"})
		return
	}

	writeJSON(w, http.StatusOK, MeResponse{User: fullUser})
}

func (h *AuthHandler) GetMiddleware() *auth.Middleware {
	return auth.NewMiddleware(h.authService)
}

func (h *AuthHandler) GetAuthService() auth.TokenService {
	return h.authService
}
