//go:build lambda

package main

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/everydev1618/colettedn/internal/handler"
)

//go:embed frontend/*
var frontendFS embed.FS

var frontendRoot fs.FS
var httpAdapterV2 *httpadapter.HandlerAdapterV2

func init() {
	h := handler.New()

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("POST /api/generate", h.GenerateDomains)
	mux.HandleFunc("POST /api/check", h.CheckAvailability)
	mux.HandleFunc("GET /api/health", h.Health)

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
