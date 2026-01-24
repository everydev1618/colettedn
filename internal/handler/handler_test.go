package handler

import "testing"

func TestIsDomainIdea(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Should be detected as domain ideas
		{"simple .com", "tonycto.com", true},
		{"simple .io", "myapp.io", true},
		{"simple .co", "startup.co", true},
		{"simple .net", "example.net", true},
		{"simple .org", "nonprofit.org", true},
		{"simple .ai", "smartbot.ai", true},
		{"simple .app", "coolapp.app", true},
		{"simple .dev", "devtools.dev", true},
		{"uppercase", "MYSITE.COM", true},
		{"mixed case", "MySite.Com", true},
		{"with spaces around", "  example.com  ", true},

		// Should NOT be detected as domain ideas
		{"project description", "a coffee shop for developers", false},
		{"domain with spaces", "my app.com", false},
		{"just TLD", ".com", false},
		{"empty string", "", false},
		{"only spaces", "   ", false},
		{"unknown TLD", "example.unknown", false},
		{"sentence with domain", "I want to build example.com as a website", false},
		{"multiple words", "cool startup idea", false},
		{"hyphenated description", "dev-tools for teams", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDomainIdea(tt.input)
			if got != tt.expected {
				t.Errorf("isDomainIdea(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsDomainIdeaEdgeCases(t *testing.T) {
	// Edge cases that might cause issues
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"just .com with prefix", "x.com", true},
		{"numbers", "123.com", true},
		{"hyphens", "my-site.com", true},
		{"multiple dots", "sub.domain.com", true},
		{"unicode", "münchen.com", true}, // Should work but might not match TLD
		{"very long", "thisisaverylongdomainnamethatmightcauseissues.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDomainIdea(tt.input)
			if got != tt.expected {
				t.Errorf("isDomainIdea(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
