//go:build lambda

package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/everydev1618/colettedn/internal/favorites"
	"github.com/everydev1618/colettedn/internal/handler"
	"github.com/everydev1618/colettedn/internal/user"
)

//go:embed frontend/*
var frontendFS embed.FS

var frontendRoot fs.FS
var httpAdapterV2 *httpadapter.HandlerAdapterV2

func init() {
	// Initialize user service (shared across handlers)
	var userService user.UserService
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		var err error
		userService, err = user.NewService("colettedn-users")
		if err != nil {
			log.Printf("[WARN] Failed to initialize user service: %v", err)
		}
	} else {
		userService = user.NewMemoryService()
	}

	h := handler.New(userService)

	// Initialize auth handler
	authHandler, err := handler.NewAuthHandler()
	if err != nil {
		log.Printf("[WARN] Failed to initialize auth handler: %v", err)
	}

	// Initialize favorites handler
	favHandler, err := handler.NewFavoritesHandler(userService)
	if err != nil {
		log.Printf("[WARN] Failed to initialize favorites handler: %v", err)
	}

	// Initialize history handler
	histHandler, err := handler.NewHistoryHandler(userService)
	if err != nil {
		log.Printf("[WARN] Failed to initialize history handler: %v", err)
	}

	// Initialize owned domains handler
	ownedHandler, err := handler.NewOwnedHandler()
	if err != nil {
		log.Printf("[WARN] Failed to initialize owned handler: %v", err)
	}

	// Initialize monitoring handler
	monitoringHandler, err := handler.NewMonitoringHandler(userService)
	if err != nil {
		log.Printf("[WARN] Failed to initialize monitoring handler: %v", err)
	}

	// Initialize billing handler
	billingHandler, err := handler.NewBillingHandler(userService)
	if err != nil {
		log.Printf("[WARN] Failed to initialize billing handler: %v", err)
	}

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("POST /api/generate", h.GenerateDomains)
	mux.HandleFunc("POST /api/check", h.CheckAvailability)
	mux.HandleFunc("POST /api/check-com", h.CheckComSite)
	mux.HandleFunc("POST /api/rdap", h.LookupRDAP)
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("GET /api/stats", h.Stats)
	mux.HandleFunc("POST /api/track/affiliate", h.TrackAffiliateClick)
	mux.HandleFunc("POST /api/track/pageview", h.TrackPageView)
	mux.HandleFunc("POST /api/track/tab-open", h.TrackTabOpen)
	mux.HandleFunc("POST /api/generate-tab-title", h.GenerateTabTitle)

	// Auth routes
	if authHandler != nil {
		mux.HandleFunc("POST /api/auth/login", authHandler.Login)
		mux.HandleFunc("GET /api/auth/verify", authHandler.Verify)
		mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)

		authMiddleware := authHandler.GetMiddleware()

		// Protected routes (require auth)
		mux.Handle("GET /api/user/me", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.Me)))
		mux.Handle("GET /api/user/preferences", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.GetPreferences)))
		mux.Handle("PUT /api/user/preferences", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.UpdatePreferences)))
		mux.Handle("PUT /api/user/monitoring-notifications", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.UpdateMonitoringNotifications)))

		if favHandler != nil {
			mux.Handle("GET /api/favorites", authMiddleware.RequireAuth(http.HandlerFunc(favHandler.List)))
			mux.Handle("POST /api/favorites", authMiddleware.RequireAuth(http.HandlerFunc(favHandler.Add)))
			mux.Handle("DELETE /api/favorites/", authMiddleware.RequireAuth(http.HandlerFunc(favHandler.Remove)))
		}

		if histHandler != nil {
			mux.Handle("GET /api/history", authMiddleware.RequireAuth(http.HandlerFunc(histHandler.List)))
			mux.Handle("POST /api/history", authMiddleware.RequireAuth(http.HandlerFunc(histHandler.Save)))
			mux.Handle("DELETE /api/history/", authMiddleware.RequireAuth(http.HandlerFunc(histHandler.Delete)))
		}

		if ownedHandler != nil {
			mux.Handle("GET /api/owned", authMiddleware.RequireAuth(http.HandlerFunc(ownedHandler.List)))
			mux.Handle("POST /api/owned", authMiddleware.RequireAuth(http.HandlerFunc(ownedHandler.Add)))
			mux.Handle("DELETE /api/owned/", authMiddleware.RequireAuth(http.HandlerFunc(ownedHandler.Remove)))
		}

		if monitoringHandler != nil {
			mux.Handle("GET /api/monitoring", authMiddleware.RequireAuth(http.HandlerFunc(monitoringHandler.List)))
			mux.Handle("POST /api/monitoring", authMiddleware.RequireAuth(http.HandlerFunc(monitoringHandler.Add)))
			mux.Handle("DELETE /api/monitoring/", authMiddleware.RequireAuth(http.HandlerFunc(monitoringHandler.Remove)))
			mux.Handle("POST /api/monitoring/", authMiddleware.RequireAuth(http.HandlerFunc(monitoringHandler.Refresh)))
		}

		// Billing routes (require auth except webhook)
		if billingHandler != nil {
			mux.Handle("POST /api/billing/checkout", authMiddleware.RequireAuth(http.HandlerFunc(billingHandler.Checkout)))
			mux.Handle("POST /api/billing/portal", authMiddleware.RequireAuth(http.HandlerFunc(billingHandler.Portal)))
			mux.HandleFunc("POST /api/billing/webhook", billingHandler.Webhook) // No auth - uses Stripe signature
		}

		// Admin routes (require admin email)
		var favService favorites.FavoritesService
		if favHandler != nil {
			favService = favHandler.GetService()
		}
		adminHandler := handler.NewAdminHandler(userService, favService)
		mux.Handle("GET /api/admin/stats", handler.RequireAdmin(authMiddleware, http.HandlerFunc(adminHandler.Stats)))
	}

	// Static files
	frontendRoot, _ = fs.Sub(frontendFS, "frontend")
	mux.Handle("GET /static/", http.FileServer(http.FS(frontendRoot)))

	httpAdapterV2 = httpadapter.NewV2(mux)
}

func handleRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Handle root and HTML pages directly to ensure correct Content-Type
	path := req.RawPath
	if path == "" || path == "/" {
		data, err := fs.ReadFile(frontendRoot, "index.html")
		if err != nil {
			return events.APIGatewayV2HTTPResponse{StatusCode: 500, Body: "Error reading index.html"}, nil
		}
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "text/html; charset=utf-8"},
			Body:       string(data),
		}, nil
	}

	// Handle welcome-pro page
	if path == "/welcome-pro" {
		data, err := fs.ReadFile(frontendRoot, "welcome-pro.html")
		if err != nil {
			return events.APIGatewayV2HTTPResponse{StatusCode: 500, Body: "Error reading welcome-pro.html"}, nil
		}
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "text/html; charset=utf-8"},
			Body:       string(data),
		}, nil
	}

	// Handle admin page (auth check happens client-side via API)
	if path == "/admin" {
		data, err := fs.ReadFile(frontendRoot, "admin.html")
		if err != nil {
			return events.APIGatewayV2HTTPResponse{StatusCode: 500, Body: "Error reading admin.html"}, nil
		}
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "text/html; charset=utf-8"},
			Body:       string(data),
		}, nil
	}

	// Handle CSS and JS files directly too
	if strings.HasPrefix(path, "/static/") {
		filePath := strings.TrimPrefix(path, "/")
		data, err := fs.ReadFile(frontendRoot, filePath)
		if err != nil {
			return events.APIGatewayV2HTTPResponse{StatusCode: 404, Body: "Not found"}, nil
		}
		contentType := "application/octet-stream"
		if strings.HasSuffix(path, ".css") {
			contentType = "text/css; charset=utf-8"
		} else if strings.HasSuffix(path, ".js") {
			contentType = "application/javascript; charset=utf-8"
		}
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": contentType},
			Body:       string(data),
		}, nil
	}

	// Use httpadapter for API routes
	return httpAdapterV2.ProxyWithContext(ctx, req)
}

func main() {
	lambda.Start(handleRequest)
}
