package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/everydev1618/colettedn/internal/handler"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	h := handler.New()

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("POST /api/generate", h.GenerateDomains)
	mux.HandleFunc("GET /api/health", h.Health)

	// Serve frontend
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("frontend/static"))))
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
