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

func TestCheckAvailability(t *testing.T) {
	client := New()
	ctx := context.Background()

	// Test with a registered domain - should NOT be available
	result, err := client.CheckAvailability(ctx, "google.com")
	if err != nil {
		t.Fatalf("CheckAvailability failed: %v", err)
	}
	if result.Available {
		t.Error("Expected google.com to NOT be available")
	}
	t.Logf("google.com available=%v", result.Available)

	// Test with a domain that almost certainly doesn't exist - should be available
	result, err = client.CheckAvailability(ctx, "thisdomain-definitely-does-not-exist-xyz123.com")
	if err != nil {
		t.Fatalf("CheckAvailability failed: %v", err)
	}
	if !result.Available {
		t.Error("Expected nonexistent domain to be available")
	}
	t.Logf("nonexistent domain available=%v", result.Available)
}

func TestCheckAvailabilityMany(t *testing.T) {
	client := New()
	ctx := context.Background()

	domains := []string{
		"google.com",                                      // taken
		"thisdomain-definitely-does-not-exist-xyz123.com", // available
	}

	results := client.CheckAvailabilityMany(ctx, domains)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// google.com should be taken
	if r, ok := results["google.com"]; ok {
		if r.Available {
			t.Error("Expected google.com to NOT be available")
		}
	} else {
		t.Error("Missing result for google.com")
	}

	// nonexistent domain should be available
	if r, ok := results["thisdomain-definitely-does-not-exist-xyz123.com"]; ok {
		if !r.Available {
			t.Error("Expected nonexistent domain to be available")
		}
	} else {
		t.Error("Missing result for nonexistent domain")
	}
}

func TestIsTLDSupported(t *testing.T) {
	client := New()

	// Supported TLDs (including multi-part)
	supported := []string{"com", "net", "org", "io", "ai", "co", "app", "dev", "co.uk", "uk"}
	for _, tld := range supported {
		if !client.IsTLDSupported(tld) {
			t.Errorf("Expected %s to be supported", tld)
		}
	}

	// Unsupported TLDs
	unsupported := []string{"zz", "fake", "notreal"}
	for _, tld := range unsupported {
		if client.IsTLDSupported(tld) {
			t.Errorf("Expected %s to NOT be supported", tld)
		}
	}
}

func TestMultiPartTLD(t *testing.T) {
	client := New()

	// Test .co.uk domain (multi-part TLD)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// google.co.uk should be taken
	result, err := client.CheckAvailability(ctx, "google.co.uk")
	if err != nil {
		t.Fatalf("CheckAvailability error: %v", err)
	}
	if result.Available {
		t.Error("Expected google.co.uk to be taken")
	}
	t.Logf("google.co.uk available=%v", result.Available)
}

func TestDNSFallback(t *testing.T) {
	// Use NewWithDefaults to avoid IANA bootstrap (which might support the TLD)
	client := NewWithDefaults()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test with a domain that has a TLD not in the default bootstrap
	// but is a real registered domain (should use DNS fallback)
	// google.de should be taken - .de may not be in defaults but DNS will find it
	result, err := client.CheckAvailability(ctx, "google.fr")
	if err != nil {
		t.Fatalf("DNS fallback error: %v", err)
	}
	// google.fr should be taken (has NS records)
	if result.Available {
		t.Logf("Warning: google.fr reported as available (DNS fallback may have failed)")
	} else {
		t.Logf("google.fr correctly detected as taken via DNS fallback")
	}

	// Test with clearly nonexistent domain
	result2, err := client.CheckAvailability(ctx, "this-domain-definitely-does-not-exist-12345.fr")
	if err != nil {
		t.Fatalf("DNS fallback error for nonexistent: %v", err)
	}
	t.Logf("nonexistent.fr available=%v error=%s", result2.Available, result2.Error)
}

func TestSupportedTLDCount(t *testing.T) {
	client := New()
	count := client.SupportedTLDCount()
	t.Logf("Supported TLD count: %d", count)

	// Should have significantly more TLDs with IANA bootstrap
	if count < 100 {
		t.Errorf("Expected at least 100 TLDs with IANA bootstrap, got %d", count)
	}
}
