package rdap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// IANA bootstrap URL for RDAP DNS services
	ianaBootstrapURL = "https://data.iana.org/rdap/dns.json"

	// Retry configuration
	maxRetries     = 3
	baseRetryDelay = 100 * time.Millisecond
	maxRetryDelay  = 2 * time.Second
)

// Client provides RDAP (Registration Data Access Protocol) lookups
type Client struct {
	client    *http.Client
	bootstrap map[string]string // TLD -> RDAP server URL
	tldList   []string          // Sorted TLDs for multi-part matching (longest first)
	mu        sync.RWMutex
}

// DomainInfo contains WHOIS-like data from RDAP
type DomainInfo struct {
	Domain         string     `json:"domain"`
	Registrar      string     `json:"registrar,omitempty"`
	CreatedDate    *time.Time `json:"createdDate,omitempty"`
	ExpirationDate *time.Time `json:"expirationDate,omitempty"`
	UpdatedDate    *time.Time `json:"updatedDate,omitempty"`
	Status         []string   `json:"status,omitempty"`    // e.g., "clientTransferProhibited"
	NameServers    []string   `json:"nameServers,omitempty"`
	DNSSEC         bool       `json:"dnssec"`
	DaysUntilExpiry *int      `json:"daysUntilExpiry,omitempty"`
	Error          string     `json:"error,omitempty"`
}

// New creates a new RDAP client with IANA bootstrap (falls back to hardcoded if fetch fails)
func New() *Client {
	bootstrap := defaultBootstrap()

	// Try to fetch IANA bootstrap for comprehensive TLD coverage
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if ianaBootstrap, err := fetchIANABootstrap(ctx); err != nil {
		log.Printf("[RDAP] Failed to fetch IANA bootstrap, using defaults: %v", err)
	} else {
		// Merge IANA bootstrap with defaults (defaults take precedence for known-good servers)
		for tld, server := range ianaBootstrap {
			if _, exists := bootstrap[tld]; !exists {
				bootstrap[tld] = server
			}
		}
		log.Printf("[RDAP] Loaded %d TLDs from IANA bootstrap (total: %d)", len(ianaBootstrap), len(bootstrap))
	}

	return &Client{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		bootstrap: bootstrap,
		tldList:   buildSortedTLDList(bootstrap),
	}
}

// NewWithDefaults creates a new RDAP client with only hardcoded defaults (no IANA fetch)
func NewWithDefaults() *Client {
	bootstrap := defaultBootstrap()
	return &Client{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		bootstrap: bootstrap,
		tldList:   buildSortedTLDList(bootstrap),
	}
}

// ianaBootstrapResponse represents the IANA RDAP bootstrap JSON structure
type ianaBootstrapResponse struct {
	Version     string       `json:"version"`
	Publication string       `json:"publication"`
	Services    [][][]string `json:"services"`
}

// fetchIANABootstrap fetches the official RDAP bootstrap from IANA
func fetchIANABootstrap(ctx context.Context) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", ianaBootstrapURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch IANA bootstrap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IANA bootstrap returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var bootstrap ianaBootstrapResponse
	if err := json.Unmarshal(body, &bootstrap); err != nil {
		return nil, fmt.Errorf("parse bootstrap JSON: %w", err)
	}

	result := make(map[string]string)
	for _, service := range bootstrap.Services {
		if len(service) < 2 {
			continue
		}
		tlds := service[0]
		servers := service[1]
		if len(servers) == 0 {
			continue
		}
		// Use the first server URL for each TLD
		serverURL := strings.TrimSuffix(servers[0], "/")
		for _, tld := range tlds {
			result[strings.ToLower(tld)] = serverURL
		}
	}

	return result, nil
}

// buildSortedTLDList creates a sorted list of TLDs for multi-part matching
// Sorted by length descending so "co.uk" matches before "uk"
func buildSortedTLDList(bootstrap map[string]string) []string {
	tlds := make([]string, 0, len(bootstrap))
	for tld := range bootstrap {
		tlds = append(tlds, tld)
	}
	sort.Slice(tlds, func(i, j int) bool {
		// Sort by length descending, then alphabetically for stability
		if len(tlds[i]) != len(tlds[j]) {
			return len(tlds[i]) > len(tlds[j])
		}
		return tlds[i] < tlds[j]
	})
	return tlds
}

// defaultBootstrap returns known RDAP servers for common TLDs
// These are verified to be reliable and take precedence over IANA bootstrap
func defaultBootstrap() map[string]string {
	return map[string]string{
		// Verisign (com, net) - very reliable
		"com": "https://rdap.verisign.com/com/v1",
		"net": "https://rdap.verisign.com/net/v1",

		// PIR (org)
		"org": "https://rdap.publicinterestregistry.org/rdap",

		// Google (app, dev, page, how)
		"app": "https://rdap.nic.google",
		"dev": "https://rdap.nic.google",

		// Identity Digital / Donuts
		"io":   "https://rdap.nic.io",
		"co":   "https://rdap.nic.co",
		"ai":   "https://rdap.nic.ai",
		"me":   "https://rdap.nic.me",
		"xyz":  "https://rdap.nic.xyz",
		"tech": "https://rdap.nic.tech",

		// Country-code TLDs (ccTLDs) commonly used
		"uk":     "https://rdap.nominet.uk/uk",
		"co.uk":  "https://rdap.nominet.uk/co.uk",
		"de":     "https://rdap.denic.de",
		"eu":     "https://rdap.eurid.eu",
		"ca":     "https://rdap.ca.fury.ca/rdap",
		"au":     "https://rdap.auda.org.au",
		"com.au": "https://rdap.auda.org.au",
		"co.za":  "https://rdap.registry.net.za/rdap",
	}
}

// Lookup queries RDAP for domain registration information
func (c *Client) Lookup(ctx context.Context, domain string) (*DomainInfo, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	tld := c.getTLDWithMultiPart(domain)
	if tld == "" {
		return nil, fmt.Errorf("invalid domain format: %s", domain)
	}

	// Get RDAP server for this TLD
	c.mu.RLock()
	server, ok := c.bootstrap[tld]
	c.mu.RUnlock()

	if !ok {
		return &DomainInfo{
			Domain: domain,
			Error:  fmt.Sprintf("RDAP not supported for .%s", tld),
		}, nil
	}

	url := fmt.Sprintf("%s/domain/%s", server, domain)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/rdap+json")

	resp, err := c.doRequestWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("RDAP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-existent domains (404 means available)
	if resp.StatusCode == http.StatusNotFound {
		return &DomainInfo{
			Domain: domain,
			Error:  "domain not found (may be available)",
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RDAP error: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseRDAPResponse(domain, body)
}

// LookupMany queries multiple domains concurrently
func (c *Client) LookupMany(ctx context.Context, domains []string) map[string]*DomainInfo {
	results := make(map[string]*DomainInfo)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrency to avoid rate limits
	sem := make(chan struct{}, 5)

	for _, domain := range domains {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			info, err := c.Lookup(ctx, d)
			mu.Lock()
			if err != nil {
				results[d] = &DomainInfo{Domain: d, Error: err.Error()}
			} else {
				results[d] = info
			}
			mu.Unlock()
		}(domain)
	}

	wg.Wait()
	return results
}

// rdapResponse represents the RDAP JSON response structure
type rdapResponse struct {
	LDHName    string       `json:"ldhName"`
	Status     []string     `json:"status"`
	Events     []rdapEvent  `json:"events"`
	Entities   []rdapEntity `json:"entities"`
	Nameservers []rdapNS    `json:"nameservers"`
	SecureDNS  *rdapDNSSEC  `json:"secureDNS"`
}

type rdapEvent struct {
	EventAction string `json:"eventAction"`
	EventDate   string `json:"eventDate"`
}

type rdapEntity struct {
	Roles      []string       `json:"roles"`
	VCardArray []interface{}  `json:"vcardArray"`
	PublicIDs  []rdapPublicID `json:"publicIds"`
}

type rdapPublicID struct {
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
}

type rdapNS struct {
	LDHName string `json:"ldhName"`
}

type rdapDNSSEC struct {
	DelegationSigned bool `json:"delegationSigned"`
}

func parseRDAPResponse(domain string, body []byte) (*DomainInfo, error) {
	var resp rdapResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse RDAP response: %w", err)
	}

	info := &DomainInfo{
		Domain: domain,
		Status: resp.Status,
	}

	// Parse events (registration, expiration, last update dates)
	for _, event := range resp.Events {
		t, err := time.Parse(time.RFC3339, event.EventDate)
		if err != nil {
			continue
		}
		switch event.EventAction {
		case "registration":
			info.CreatedDate = &t
		case "expiration":
			info.ExpirationDate = &t
			// Calculate days until expiry
			days := int(time.Until(t).Hours() / 24)
			info.DaysUntilExpiry = &days
		case "last changed":
			info.UpdatedDate = &t
		}
	}

	// Find registrar from entities
	for _, entity := range resp.Entities {
		for _, role := range entity.Roles {
			if role == "registrar" {
				info.Registrar = extractRegistrarName(entity)
				break
			}
		}
	}

	// Extract nameservers
	for _, ns := range resp.Nameservers {
		if ns.LDHName != "" {
			info.NameServers = append(info.NameServers, strings.ToLower(ns.LDHName))
		}
	}

	// Check DNSSEC
	if resp.SecureDNS != nil {
		info.DNSSEC = resp.SecureDNS.DelegationSigned
	}

	return info, nil
}

// extractRegistrarName tries to get a human-readable registrar name
func extractRegistrarName(entity rdapEntity) string {
	// Try to get IANA ID first
	for _, pid := range entity.PublicIDs {
		if pid.Type == "IANA Registrar ID" {
			// Could map to registrar name, but ID is useful too
			return fmt.Sprintf("IANA ID: %s", pid.Identifier)
		}
	}

	// Try to extract from vCard
	if len(entity.VCardArray) >= 2 {
		if props, ok := entity.VCardArray[1].([]interface{}); ok {
			for _, prop := range props {
				if arr, ok := prop.([]interface{}); ok && len(arr) >= 4 {
					if arr[0] == "fn" {
						if name, ok := arr[3].(string); ok {
							return name
						}
					}
				}
			}
		}
	}

	return ""
}

// getTLD extracts the TLD from a domain, handling multi-part TLDs like co.uk
func getTLD(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

// getTLDWithMultiPart extracts the TLD from a domain, checking against known multi-part TLDs
func (c *Client) getTLDWithMultiPart(domain string) string {
	domain = strings.ToLower(domain)

	c.mu.RLock()
	tldList := c.tldList
	c.mu.RUnlock()

	// Check against sorted TLD list (longest first)
	for _, tld := range tldList {
		suffix := "." + tld
		if strings.HasSuffix(domain, suffix) {
			// Make sure there's something before the TLD
			prefix := strings.TrimSuffix(domain, suffix)
			if prefix != "" && !strings.HasSuffix(prefix, ".") {
				return tld
			}
		}
	}

	// Fallback to simple extraction
	return getTLD(domain)
}

// doRequestWithRetry performs an HTTP request with exponential backoff retry
func (c *Client) doRequestWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			delay := baseRetryDelay * time.Duration(1<<uint(attempt-1))
			if delay > maxRetryDelay {
				delay = maxRetryDelay
			}
			// Add jitter (0-25% of delay)
			jitter := time.Duration(rand.Int63n(int64(delay / 4)))
			delay += jitter

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			// Clone the request for retry (body already read)
			req = req.Clone(ctx)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			// Retry on network errors
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}

		// Don't retry on success or client errors (4xx)
		// Do retry on server errors (5xx) except 501 Not Implemented
		if resp.StatusCode < 500 || resp.StatusCode == 501 {
			return resp, nil
		}

		// Server error - close body and retry
		resp.Body.Close()
		lastErr = fmt.Errorf("server error: status %d", resp.StatusCode)
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// IsExpiringSoon returns true if domain expires within the given days
func (d *DomainInfo) IsExpiringSoon(withinDays int) bool {
	if d.DaysUntilExpiry == nil {
		return false
	}
	return *d.DaysUntilExpiry <= withinDays && *d.DaysUntilExpiry > 0
}

// IsExpired returns true if domain has already expired
func (d *DomainInfo) IsExpired() bool {
	if d.DaysUntilExpiry == nil {
		return false
	}
	return *d.DaysUntilExpiry < 0
}

// AvailabilityResult represents the availability status of a domain
type AvailabilityResult struct {
	Domain    string `json:"domain"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// CheckAvailability checks if a single domain is available for registration
// Returns available=true if RDAP returns 404 (not found)
// Falls back to DNS NS record check if RDAP doesn't support the TLD
func (c *Client) CheckAvailability(ctx context.Context, domain string) (*AvailabilityResult, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	tld := c.getTLDWithMultiPart(domain)
	if tld == "" {
		return nil, fmt.Errorf("invalid domain format: %s", domain)
	}

	// Get RDAP server for this TLD
	c.mu.RLock()
	server, ok := c.bootstrap[tld]
	c.mu.RUnlock()

	if !ok {
		// Fallback to DNS-based availability check
		return c.checkAvailabilityDNS(ctx, domain, tld)
	}

	url := fmt.Sprintf("%s/domain/%s", server, domain)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/rdap+json")

	resp, err := c.doRequestWithRetry(ctx, req)
	if err != nil {
		// RDAP failed - try DNS fallback
		log.Printf("[RDAP] Request failed for %s, trying DNS fallback: %v", domain, err)
		return c.checkAvailabilityDNS(ctx, domain, tld)
	}
	defer resp.Body.Close()

	// 404 = domain not found = available
	if resp.StatusCode == http.StatusNotFound {
		return &AvailabilityResult{
			Domain:    domain,
			Available: true,
		}, nil
	}

	// 200 = domain exists = taken
	if resp.StatusCode == http.StatusOK {
		return &AvailabilityResult{
			Domain:    domain,
			Available: false,
		}, nil
	}

	// Other status codes - try DNS fallback
	log.Printf("[RDAP] Unexpected status %d for %s, trying DNS fallback", resp.StatusCode, domain)
	return c.checkAvailabilityDNS(ctx, domain, tld)
}

// checkAvailabilityDNS checks domain availability using DNS NS record lookup
// This is a fallback for TLDs not supported by RDAP or when RDAP fails
// Logic: If a domain has NS records, it's registered (taken)
// Note: This is a heuristic - some registered domains may not have NS records
func (c *Client) checkAvailabilityDNS(ctx context.Context, domain, tld string) (*AvailabilityResult, error) {
	// Create a resolver with timeout from context
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, network, address)
		},
	}

	// Look up NS records for the domain
	ns, err := resolver.LookupNS(ctx, domain)
	if err != nil {
		// DNS error could mean domain doesn't exist (available) or network issue
		var dnsErr *net.DNSError
		if ok := errors.As(err, &dnsErr); ok {
			if dnsErr.IsNotFound {
				// NXDOMAIN - domain doesn't exist, likely available
				return &AvailabilityResult{
					Domain:    domain,
					Available: true,
				}, nil
			}
		}
		// Other DNS errors - can't determine availability
		return &AvailabilityResult{
			Domain: domain,
			Error:  fmt.Sprintf("DNS lookup failed for .%s: %v", tld, err),
		}, nil
	}

	// Has NS records = domain is registered (taken)
	if len(ns) > 0 {
		return &AvailabilityResult{
			Domain:    domain,
			Available: false,
		}, nil
	}

	// No NS records - could be available or just not delegated
	// Return as potentially available with a note
	return &AvailabilityResult{
		Domain:    domain,
		Available: true,
	}, nil
}

// CheckAvailabilityMany checks multiple domains concurrently
func (c *Client) CheckAvailabilityMany(ctx context.Context, domains []string) map[string]*AvailabilityResult {
	results := make(map[string]*AvailabilityResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrency to avoid rate limits
	sem := make(chan struct{}, 10)

	for _, domain := range domains {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := c.CheckAvailability(ctx, d)
			mu.Lock()
			if err != nil {
				results[d] = &AvailabilityResult{Domain: d, Error: err.Error()}
			} else {
				results[d] = result
			}
			mu.Unlock()
		}(domain)
	}

	wg.Wait()
	return results
}

// IsTLDSupported returns true if RDAP lookups are supported for the given TLD
func (c *Client) IsTLDSupported(tld string) bool {
	tld = strings.ToLower(strings.TrimPrefix(tld, "."))
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.bootstrap[tld]
	return ok
}

// SupportedTLDCount returns the number of TLDs supported by this client
func (c *Client) SupportedTLDCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.bootstrap)
}
