package generator

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestParseCategorizedDomains(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNil  bool
		wantCats []string
	}{
		{
			name: "valid JSON",
			input: `{
				"Professional": ["example.com", "corp.io"],
				"Playful": ["fun.co"],
				"Creative": ["neat.dev"],
				"Minimal": ["x.ai"]
			}`,
			wantNil:  false,
			wantCats: []string{"Professional", "Playful", "Creative", "Minimal"},
		},
		{
			name:     "JSON with surrounding text",
			input:    `Here are some domains: {"Professional": ["test.com"]} Hope you like them!`,
			wantNil:  false,
			wantCats: []string{"Professional"},
		},
		{
			name:     "empty response",
			input:    "",
			wantNil:  true,
			wantCats: nil,
		},
		{
			name:     "no JSON braces",
			input:    "Just some text without JSON",
			wantNil:  true,
			wantCats: nil,
		},
		{
			name:     "invalid JSON",
			input:    `{"broken: json}`,
			wantNil:  true,
			wantCats: nil,
		},
		{
			name:     "empty JSON object",
			input:    `{}`,
			wantNil:  false,
			wantCats: []string{},
		},
		{
			name:     "domains without TLD filtered out",
			input:    `{"Test": ["valid.com", "invalid", "also-valid.io"]}`,
			wantNil:  false,
			wantCats: []string{"Test"},
		},
		{
			name:     "whitespace and case normalization",
			input:    `{"Test": ["  UPPER.COM  ", "lower.io"]}`,
			wantNil:  false,
			wantCats: []string{"Test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCategorizedDomains(tt.input)

			if tt.wantNil && result != nil {
				t.Errorf("expected nil result, got %v", result)
				return
			}
			if !tt.wantNil && result == nil {
				t.Error("expected non-nil result, got nil")
				return
			}
			if result == nil {
				return
			}

			// Check expected categories exist
			for _, cat := range tt.wantCats {
				if _, ok := result[cat]; !ok {
					t.Errorf("expected category %q to exist", cat)
				}
			}
		})
	}
}

func TestParseCategorizedDomainsFiltering(t *testing.T) {
	input := `{"Test": ["valid.com", "no-tld", "  MIXED.IO  ", "", "   "]}`
	result := parseCategorizedDomains(input)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	domains := result["Test"]
	if len(domains) != 2 {
		t.Errorf("expected 2 valid domains, got %d: %v", len(domains), domains)
	}

	// Check normalization
	for _, d := range domains {
		if d != "valid.com" && d != "mixed.io" {
			t.Errorf("unexpected domain: %s", d)
		}
	}
}

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
