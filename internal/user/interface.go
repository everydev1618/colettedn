package user

import "context"

// UserStats contains aggregate user statistics
type UserStats struct {
	TotalUsers  int     `json:"totalUsers"`
	ProUsers    int     `json:"proUsers"`
	FreeUsers   int     `json:"freeUsers"`
	MRR         float64 `json:"mrr"` // Monthly Recurring Revenue in dollars
}

// UserService defines the interface for user operations
type UserService interface {
	GetByID(ctx context.Context, userID string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, email string) (*User, error)
	GetOrCreate(ctx context.Context, email string) (*User, error)
	UpdateSubscription(ctx context.Context, userID, stripeCustomerID string, tier SubscriptionTier, expiry int64) error
	UpdatePreferences(ctx context.Context, userID string, preferredRegistrar string, preferredOtherRegistrar string, theme string) error
	UpdateMonitoringNotifications(ctx context.Context, userID string, enabled bool) error
	GetByStripeCustomerID(ctx context.Context, customerID string) (*User, error)
	GetStats(ctx context.Context) (*UserStats, error)
	ListProUsers(ctx context.Context, limit int) ([]*User, error)
	ListRecentUsers(ctx context.Context, limit int) ([]*User, error)
}

// Ensure implementations satisfy the interface
var _ UserService = (*Service)(nil)
var _ UserService = (*MemoryService)(nil)
