package tools

import (
	"context"
	"testing"

	"github.com/everydev1618/colettedn/internal/rdap"
)

func TestCheckDomainTool_InvalidDomain(t *testing.T) {
	client := rdap.NewWithDefaults()
	tool := NewCheckDomainTool(client)

	// Empty domain should error
	_, err := tool.Execute(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty domain")
	}

	// No TLD should error
	_, err = tool.Execute(context.Background(), "nodots")
	if err == nil {
		t.Fatal("expected error for domain without TLD")
	}
}

func TestCheckDomainsTool_InvalidInput(t *testing.T) {
	client := rdap.NewWithDefaults()
	tool := NewCheckDomainsTool(client)

	// Empty list should error
	_, err := tool.Execute(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error for empty domain list")
	}
}

func TestCheckDomainTool_KnownTaken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := rdap.NewWithDefaults()
	tool := NewCheckDomainTool(client)

	result, err := tool.Execute(context.Background(), "google.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Available {
		t.Fatal("google.com should not be available")
	}
}

func TestCheckDomainsTool_Batch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := rdap.NewWithDefaults()
	tool := NewCheckDomainsTool(client)

	results, err := tool.Execute(context.Background(), []string{"google.com", "facebook.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Available {
			t.Fatalf("%s should not be available", r.Domain)
		}
	}
}
