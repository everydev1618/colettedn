package rdap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client provides RDAP (Registration Data Access Protocol) lookups
type Client struct {
	client    *http.Client
	bootstrap map[string]string // TLD -> RDAP server URL
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

// New creates a new RDAP client
func New() *Client {
	return &Client{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		bootstrap: defaultBootstrap(),
	}
}

// defaultBootstrap returns known RDAP servers for common TLDs
// Full bootstrap can be fetched from https://data.iana.org/rdap/dns.json
func defaultBootstrap() map[string]string {
	return map[string]string{
		// Verisign (com, net)
		"com": "https://rdap.verisign.com/com/v1",
		"net": "https://rdap.verisign.com/net/v1",

		// PIR (org)
		"org": "https://rdap.publicinterestregistry.org/rdap",

		// Donuts (many new gTLDs)
		"io":  "https://rdap.nic.io",
		"app": "https://rdap.nic.google",
		"dev": "https://rdap.nic.google",

		// Identity Digital (formerly Donuts)
		"co": "https://rdap.nic.co",
		"ai": "https://rdap.nic.ai",

		// Others
		"me":   "https://rdap.nic.me",
		"xyz":  "https://rdap.nic.xyz",
		"tech": "https://rdap.nic.tech",
	}
}

// Lookup queries RDAP for domain registration information
func (c *Client) Lookup(ctx context.Context, domain string) (*DomainInfo, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	tld := getTLD(domain)
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

	resp, err := c.client.Do(req)
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

func getTLD(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
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
