package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	apiUser  string
	apiKey   string
	username string
	clientIP string
	baseURL  string
	client   *http.Client
	sandbox  bool
}

type Config struct {
	APIUser  string
	APIKey   string
	Username string
	ClientIP string
	Sandbox  bool
}

func New(cfg Config) *Client {
	baseURL := "https://api.namecheap.com/xml.response"
	if cfg.Sandbox {
		baseURL = "https://api.sandbox.namecheap.com/xml.response"
	}

	return &Client{
		apiUser:  cfg.APIUser,
		apiKey:   cfg.APIKey,
		username: cfg.Username,
		clientIP: cfg.ClientIP,
		baseURL:  baseURL,
		client:   &http.Client{},
		sandbox:  cfg.Sandbox,
	}
}

type apiResponse struct {
	XMLName         xml.Name        `xml:"ApiResponse"`
	Status          string          `xml:"Status,attr"`
	Errors          []apiError      `xml:"Errors>Error"`
	CommandResponse commandResponse `xml:"CommandResponse"`
}

type apiError struct {
	Number  int    `xml:"Number,attr"`
	Message string `xml:",chardata"`
}

type commandResponse struct {
	DomainCheckResults []domainCheckResult `xml:"DomainCheckResult"`
	UserGetPricingResult *userGetPricingResult `xml:"UserGetPricingResult"`
}

type domainCheckResult struct {
	Domain                   string  `xml:"Domain,attr"`
	Available                bool    `xml:"Available,attr"`
	IsPremium                bool    `xml:"IsPremiumName,attr"`
	PremiumRegistrationPrice float64 `xml:"PremiumRegistrationPrice,attr"`
	PremiumRenewalPrice      float64 `xml:"PremiumRenewalPrice,attr"`
	IcannFee                 float64 `xml:"IcannFee,attr"`
}

type userGetPricingResult struct {
	ProductType []productType `xml:"ProductType"`
}

type productType struct {
	Name        string        `xml:"Name,attr"`
	ProductCategory []productCategory `xml:"ProductCategory"`
}

type productCategory struct {
	Name    string    `xml:"Name,attr"`
	Product []product `xml:"Product"`
}

type product struct {
	Name  string  `xml:"Name,attr"`
	Price []price `xml:"Price"`
}

type price struct {
	Duration     int     `xml:"Duration,attr"`
	Type         string  `xml:"Type,attr"`
	Price        float64 `xml:"Price,attr"`
	RegularPrice float64 `xml:"RegularPrice,attr"`
}

type DomainStatus struct {
	Domain    string   `json:"domain"`
	Available bool     `json:"available"`
	IsPremium bool     `json:"isPremium"`
	Price     *float64 `json:"price,omitempty"`
	IcannFee  *float64 `json:"icannFee,omitempty"`
	Error     string   `json:"error,omitempty"`
}

func (c *Client) CheckAvailability(ctx context.Context, domains []string) ([]DomainStatus, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("Namecheap API not configured")
	}

	// API supports up to 50 domains per request
	if len(domains) > 50 {
		domains = domains[:50]
	}

	params := url.Values{}
	params.Set("ApiUser", c.apiUser)
	params.Set("ApiKey", c.apiKey)
	params.Set("UserName", c.username)
	params.Set("ClientIp", c.clientIP)
	params.Set("Command", "namecheap.domains.check")
	params.Set("DomainList", strings.Join(domains, ","))

	reqURL := c.baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp apiResponse
	if err := xml.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if apiResp.Status != "OK" {
		if len(apiResp.Errors) > 0 {
			return nil, fmt.Errorf("API error: %s", apiResp.Errors[0].Message)
		}
		return nil, fmt.Errorf("API error: status %s", apiResp.Status)
	}

	// Get TLD pricing for non-premium domains
	tldPrices := c.getTLDPrices()

	results := make([]DomainStatus, len(apiResp.CommandResponse.DomainCheckResults))
	for i, r := range apiResp.CommandResponse.DomainCheckResults {
		results[i] = DomainStatus{
			Domain:    r.Domain,
			Available: r.Available,
			IsPremium: r.IsPremium,
		}

		if r.Available {
			if r.IsPremium && r.PremiumRegistrationPrice > 0 {
				price := r.PremiumRegistrationPrice
				results[i].Price = &price
			} else {
				// Use TLD pricing
				tld := getTLD(r.Domain)
				if p, ok := tldPrices[tld]; ok {
					results[i].Price = &p
				}
			}

			if r.IcannFee > 0 {
				results[i].IcannFee = &r.IcannFee
			}
		}
	}

	return results, nil
}

func (c *Client) getTLDPrices() map[string]float64 {
	// Fallback prices (approximate Namecheap retail)
	return map[string]float64{
		"com": 10.98,
		"co":  11.98,
		"net": 12.98,
		"org": 12.98,
		"io":  32.98,
		"ai":  79.98,
		"dev": 15.98,
		"app": 15.98,
	}
}

func getTLD(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) > 1 {
		return strings.ToLower(parts[len(parts)-1])
	}
	return ""
}

func (c *Client) IsConfigured() bool {
	return c.apiKey != ""
}
