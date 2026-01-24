//go:build lambda

package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/everydev1618/colettedn/internal/monitoring"
	"github.com/everydev1618/colettedn/internal/notifier"
	"github.com/everydev1618/colettedn/internal/user"
)

func main() {
	lambda.Start(handler)
}

func handler(ctx context.Context) error {
	log.Println("[NOTIFIER] Lambda invoked")

	// Initialize services
	userService, err := user.NewService("colettedn-users")
	if err != nil {
		log.Printf("[NOTIFIER] Failed to initialize user service: %v", err)
		return err
	}

	monitoringService, err := monitoring.NewService("colettedn-monitoring")
	if err != nil {
		log.Printf("[NOTIFIER] Failed to initialize monitoring service: %v", err)
		return err
	}

	fromEmail := os.Getenv("FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = "noreply@colettedn.com"
	}

	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "https://colettedn.com"
	}

	// Create notifier and run
	n, err := notifier.New(monitoringService, userService, fromEmail, appURL)
	if err != nil {
		log.Printf("[NOTIFIER] Failed to create notifier: %v", err)
		return err
	}

	if err := n.Run(ctx); err != nil {
		log.Printf("[NOTIFIER] Failed to run notifier: %v", err)
		return err
	}

	log.Println("[NOTIFIER] Lambda completed successfully")
	return nil
}
