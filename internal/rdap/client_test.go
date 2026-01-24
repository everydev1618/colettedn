package rdap

import (
	"context"
	"testing"
	"time"
)

func TestLookup(t *testing.T) {
	client := New()
	ctx := context.Background()

	// Test with a well-known domain
	info, err := client.Lookup(ctx, "google.com")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}

	if info.Domain != "google.com" {
		t.Errorf("Expected domain google.com, got %s", info.Domain)
	}

	if info.CreatedDate == nil {
		t.Error("Expected creation date for google.com")
	}

	if info.ExpirationDate == nil {
		t.Error("Expected expiration date for google.com")
	}

	if len(info.NameServers) == 0 {
		t.Error("Expected nameservers for google.com")
	}

	t.Logf("google.com info: registrar=%s, created=%v, expires=%v, nameservers=%v",
		info.Registrar, info.CreatedDate, info.ExpirationDate, info.NameServers)
}

func TestLookupNonexistent(t *testing.T) {
	client := New()
	ctx := context.Background()

	// Test with a domain that almost certainly doesn't exist
	info, err := client.Lookup(ctx, "thisdomain-definitely-does-not-exist-12345.com")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}

	if info.Error == "" {
		t.Error("Expected error for nonexistent domain")
	}

	t.Logf("Nonexistent domain error: %s", info.Error)
}

func TestLookupUnsupportedTLD(t *testing.T) {
	client := New()
	ctx := context.Background()

	// Test with an unsupported TLD
	info, err := client.Lookup(ctx, "example.zz")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}

	if info.Error == "" {
		t.Error("Expected error for unsupported TLD")
	}

	t.Logf("Unsupported TLD error: %s", info.Error)
}

func TestIsExpiringSoon(t *testing.T) {
	// Domain expiring in 30 days
	days := 30
	info := &DomainInfo{
		Domain:          "test.com",
		DaysUntilExpiry: &days,
	}

	if !info.IsExpiringSoon(60) {
		t.Error("Expected domain expiring in 30 days to be flagged as expiring within 60 days")
	}

	if info.IsExpiringSoon(15) {
		t.Error("Expected domain expiring in 30 days NOT to be flagged as expiring within 15 days")
	}
}

func TestIsExpired(t *testing.T) {
	// Domain expired 5 days ago
	days := -5
	info := &DomainInfo{
		Domain:          "test.com",
		DaysUntilExpiry: &days,
	}

	if !info.IsExpired() {
		t.Error("Expected domain with negative days to be flagged as expired")
	}

	// Domain expiring in 30 days
	days = 30
	info.DaysUntilExpiry = &days
	if info.IsExpired() {
		t.Error("Expected domain with positive days NOT to be flagged as expired")
	}
}

func TestCache(t *testing.T) {
	cache := NewCache(time.Hour)

	info := &DomainInfo{
		Domain:    "test.com",
		Registrar: "Test Registrar",
	}

	// Test Set and Get
	cache.Set("test.com", info)

	cached, ok := cache.Get("test.com")
	if !ok {
		t.Fatal("Expected to find cached entry")
	}

	if cached.Domain != "test.com" {
		t.Errorf("Expected domain test.com, got %s", cached.Domain)
	}

	if !cached.FromCache {
		t.Error("Expected FromCache to be true")
	}

	// Test cache miss
	_, ok = cache.Get("notcached.com")
	if ok {
		t.Error("Expected cache miss for uncached domain")
	}

	// Test Size
	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", cache.Size())
	}
}

func TestCacheExpiry(t *testing.T) {
	// Use very short TTL for testing
	cache := NewCache(10 * time.Millisecond)

	info := &DomainInfo{Domain: "test.com"}
	cache.Set("test.com", info)

	// Should be found immediately
	_, ok := cache.Get("test.com")
	if !ok {
		t.Fatal("Expected to find entry immediately after set")
	}

	// Wait for expiry
	time.Sleep(20 * time.Millisecond)

	// Should be expired now
	_, ok = cache.Get("test.com")
	if ok {
		t.Error("Expected entry to be expired")
	}
}

func TestGetMany(t *testing.T) {
	cache := NewCache(time.Hour)

	// Pre-populate some entries
	cache.Set("cached1.com", &DomainInfo{Domain: "cached1.com"})
	cache.Set("cached2.com", &DomainInfo{Domain: "cached2.com"})

	domains := []string{"cached1.com", "uncached.com", "cached2.com"}
	cached, uncached := cache.GetMany(domains)

	if len(cached) != 2 {
		t.Errorf("Expected 2 cached, got %d", len(cached))
	}

	if len(uncached) != 1 {
		t.Errorf("Expected 1 uncached, got %d", len(uncached))
	}

	if uncached[0] != "uncached.com" {
		t.Errorf("Expected uncached.com in uncached list, got %s", uncached[0])
	}
}

func TestLookupMany(t *testing.T) {
	client := New()
	ctx := context.Background()

	domains := []string{"google.com", "example.com"}
	results := client.LookupMany(ctx, domains)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	for _, domain := range domains {
		if _, ok := results[domain]; !ok {
			t.Errorf("Missing result for %s", domain)
		}
	}
}
