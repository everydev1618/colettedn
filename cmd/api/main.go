package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/everydev1618/colettedn/internal/favorites"
	"github.com/everydev1618/colettedn/internal/handler"
	"github.com/everydev1618/colettedn/internal/user"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if present
	godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize user service (in-memory for local dev)
	userService := user.NewMemoryService()

	h := handler.New(userService)

	// Initialize auth handler
	authHandler, err := handler.NewAuthHandler()
	if err != nil {
		log.Printf("[WARN] Failed to initialize auth handler: %v", err)
	}

	// Initialize favorites handler
	favHandler, err := handler.NewFavoritesHandler()
	if err != nil {
		log.Printf("[WARN] Failed to initialize favorites handler: %v", err)
	}

	// Initialize history handler
	histHandler, err := handler.NewHistoryHandler()
	if err != nil {
		log.Printf("[WARN] Failed to initialize history handler: %v", err)
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
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("GET /api/stats", h.Stats)
	mux.HandleFunc("POST /api/track/affiliate", h.TrackAffiliateClick)

	// Auth routes
	if authHandler != nil {
		mux.HandleFunc("POST /api/auth/login", authHandler.Login)
		mux.HandleFunc("GET /api/auth/verify", authHandler.Verify)
		mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)

		authMiddleware := authHandler.GetMiddleware()

		// Protected routes (require auth)
		mux.Handle("GET /api/user/me", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.Me)))

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

		// Billing routes (require auth except webhook)
		if billingHandler != nil {
			mux.Handle("POST /api/billing/checkout", authMiddleware.RequireAuth(http.HandlerFunc(billingHandler.Checkout)))
			mux.Handle("POST /api/billing/portal", authMiddleware.RequireAuth(http.HandlerFunc(billingHandler.Portal)))
			mux.HandleFunc("POST /api/billing/webhook", billingHandler.Webhook)
		}

		// Admin routes (require admin email)
		var favService favorites.FavoritesService
		if favHandler != nil {
			favService = favHandler.GetService()
		}
		adminHandler := handler.NewAdminHandler(userService, favService)
		mux.Handle("GET /admin", handler.RequireAdmin(authMiddleware, http.HandlerFunc(adminHandler.Dashboard)))
		mux.Handle("GET /api/admin/stats", handler.RequireAdmin(authMiddleware, http.HandlerFunc(adminHandler.Stats)))
	}

	// Serve frontend
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("frontend/static"))))
	mux.HandleFunc("GET /welcome-pro", h.ServeWelcomePro)
	mux.HandleFunc("GET /", h.ServeIndex)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	log.Printf("Colette DN starting on http://localhost:%s", port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
