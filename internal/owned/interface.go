package owned

import "context"

// AcquisitionType indicates how the user acquired the domain
type AcquisitionType string

const (
	// AcquisitionPreviouslyOwned means the user owned the domain before using ColetteDN
	AcquisitionPreviouslyOwned AcquisitionType = "previously_owned"
	// AcquisitionFoundViaColette means the user found and registered the domain via ColetteDN
	AcquisitionFoundViaColette AcquisitionType = "found_via_colette"
)

// OwnedDomain represents a domain owned by a user
type OwnedDomain struct {
	UserID          string          `dynamodbav:"user_id" json:"userId"`
	Domain          string          `dynamodbav:"domain" json:"domain"`
	AcquisitionType AcquisitionType `dynamodbav:"acquisition_type" json:"acquisitionType"`
	CreatedAt       int64           `dynamodbav:"created_at" json:"createdAt"`
}

// OwnedService defines the interface for owned domain operations
type OwnedService interface {
	Add(ctx context.Context, userID, domain string, acquisitionType AcquisitionType) (*OwnedDomain, error)
	Remove(ctx context.Context, userID, domain string) error
	List(ctx context.Context, userID string) ([]OwnedDomain, error)
	IsOwned(ctx context.Context, userID, domain string) (*OwnedDomain, error)
	GetTotalCount(ctx context.Context) (int64, error)
	GetCountByType(ctx context.Context) (map[AcquisitionType]int64, error)
	ListRecent(ctx context.Context, limit int) ([]OwnedDomain, error)
}

// Ensure implementations satisfy the interface
var _ OwnedService = (*Service)(nil)
var _ OwnedService = (*MemoryService)(nil)
