package handler

import (
	"testing"
)

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

func TestScoreDomain(t *testing.T) {
	tests := []struct {
		domain   string
		minScore int
		maxScore int
		desc     string
	}{
		// Ideal domains should score high (85+)
		{"acme.com", 90, 100, "short .com domain"},
		{"stripe.com", 90, 100, "5-letter .com"},
		{"notion.com", 90, 100, "6-letter .com"},

		// Good domains with non-.com TLDs (85-95)
		{"acme.io", 85, 100, "short .io domain"},
		{"devtools.app", 85, 100, "longer .app domain"},
		{"startup.co", 85, 100, "short .co domain"},

		// Penalized domains (still decent scores due to good TLDs)
		{"my-site.com", 75, 90, "hyphen penalty"},
		{"app123.com", 70, 85, "number penalty"},
		{"xyzqrst.com", 65, 85, "low vowels penalty"},
		{"verylongdomainname.com", 65, 85, "longer name"},

		// Edge cases
		{"a.io", 50, 75, "very short (too short)"},
		{"thisisareallylongdomainnamethatkeepsgoing.xyz", 50, 75, "extremely long with lesser TLD"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			score := scoreDomain(tt.domain)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("scoreDomain(%q) = %d, want between %d-%d", tt.domain, score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestScoreDomainComponents(t *testing.T) {
	// Test that .com scores higher than other TLDs for same name
	comScore := scoreDomain("acme.com")
	ioScore := scoreDomain("acme.io")
	xyzScore := scoreDomain("acme.xyz")

	if comScore <= ioScore {
		t.Errorf(".com (%d) should score higher than .io (%d)", comScore, ioScore)
	}
	if ioScore <= xyzScore {
		t.Errorf(".io (%d) should score higher than .xyz (%d)", ioScore, xyzScore)
	}

	// Test that shorter domains score higher
	shortScore := scoreDomain("acme.com")
	longScore := scoreDomain("acmecorporation.com")
	if shortScore <= longScore {
		t.Errorf("shorter domain (%d) should score higher than longer (%d)", shortScore, longScore)
	}

	// Test hyphen penalty
	cleanScore := scoreDomain("mysite.com")
	hyphenScore := scoreDomain("my-site.com")
	if cleanScore <= hyphenScore {
		t.Errorf("clean domain (%d) should score higher than hyphenated (%d)", cleanScore, hyphenScore)
	}
}
