package namecheap

import "testing"

func TestGetTLD(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected string
	}{
		{"standard .com", "example.com", "com"},
		{"standard .io", "myapp.io", "io"},
		{"subdomain", "www.example.com", "com"},
		{"uppercase", "EXAMPLE.COM", "com"},
		{"mixed case", "Example.IO", "io"},
		{"long TLD", "example.coffee", "coffee"},
		{"empty string", "", ""},
		{"no TLD", "localhost", ""},
		{"just TLD", ".com", "com"},
		{"double dot", "example..com", "com"},
		{"trailing dot", "example.com.", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTLD(tt.domain)
			if got != tt.expected {
				t.Errorf("getTLD(%q) = %q, want %q", tt.domain, got, tt.expected)
			}
		})
	}
}

func TestClientIsConfigured(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		c := New(Config{})
		if c.IsConfigured() {
			t.Error("expected IsConfigured() = false for empty config")
		}
	})

	t.Run("configured", func(t *testing.T) {
		c := New(Config{APIKey: "test-key"})
		if !c.IsConfigured() {
			t.Error("expected IsConfigured() = true when API key set")
		}
	})
}

func TestNewClient(t *testing.T) {
	t.Run("production URL", func(t *testing.T) {
		c := New(Config{Sandbox: false})
		if c.baseURL != "https://api.namecheap.com/xml.response" {
			t.Errorf("expected production URL, got %s", c.baseURL)
		}
	})

	t.Run("sandbox URL", func(t *testing.T) {
		c := New(Config{Sandbox: true})
		if c.baseURL != "https://api.sandbox.namecheap.com/xml.response" {
			t.Errorf("expected sandbox URL, got %s", c.baseURL)
		}
	})
}
