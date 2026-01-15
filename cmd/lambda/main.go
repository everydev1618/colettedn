//go:build lambda

package main

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/everydev1618/colettedn/internal/handler"
)

//go:embed frontend/*
var frontendFS embed.FS

func main() {
	h := handler.New()

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("POST /api/generate", h.GenerateDomains)
	mux.HandleFunc("POST /api/check", h.CheckAvailability)
	mux.HandleFunc("GET /api/health", h.Health)

	// Serve embedded frontend
	frontendRoot, _ := fs.Sub(frontendFS, "frontend")
	mux.Handle("GET /static/", http.FileServer(http.FS(frontendRoot)))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, _ := fs.ReadFile(frontendRoot, "index.html")
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})

	lambda.Start(httpadapter.New(mux).ProxyWithContext)
}
