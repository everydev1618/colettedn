package tools

import (
	"context"
	"fmt"

	"github.com/everydev1618/colettedn/internal/rdap"
)

// CheckDomainTool wraps the RDAP client for single domain availability checks.
type CheckDomainTool struct {
	client *rdap.Client
}

func NewCheckDomainTool(client *rdap.Client) *CheckDomainTool {
	return &CheckDomainTool{client: client}
}

func (t *CheckDomainTool) Execute(ctx context.Context, domain string) (*rdap.AvailabilityResult, error) {
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	return t.client.CheckAvailability(ctx, domain)
}

// CheckDomainsTool wraps the RDAP client for batch domain availability checks.
type CheckDomainsTool struct {
	client *rdap.Client
}

func NewCheckDomainsTool(client *rdap.Client) *CheckDomainsTool {
	return &CheckDomainsTool{client: client}
}

func (t *CheckDomainsTool) Execute(ctx context.Context, domains []string) ([]*rdap.AvailabilityResult, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("at least one domain is required")
	}

	resultsMap := t.client.CheckAvailabilityMany(ctx, domains)

	results := make([]*rdap.AvailabilityResult, 0, len(domains))
	for _, domain := range domains {
		if r, ok := resultsMap[domain]; ok {
			results = append(results, r)
		}
	}
	return results, nil
}
