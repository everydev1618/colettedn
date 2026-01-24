package generator

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestModelSmoke verifies the configured model ID is valid by making a minimal API call.
// This catches model ID typos or deprecated model names before deployment.
// Requires ANTHROPIC_API_KEY environment variable.
func TestModelSmoke(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping smoke test")
	}

	g := New(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Minimal prompt to verify model works - should be fast and cheap
	result, err := g.GenerateCategorized(ctx, "test", []string{"com"})
	if err != nil {
		t.Fatalf("API call failed (likely invalid model ID): %v", err)
	}

	if len(result) == 0 {
		t.Fatal("Expected non-empty result from API")
	}
}
