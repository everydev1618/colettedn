package user

import "context"

// UserService defines the interface for user operations
type UserService interface {
	GetByID(ctx context.Context, userID string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, email string) (*User, error)
	GetOrCreate(ctx context.Context, email string) (*User, error)
	UpdateSubscription(ctx context.Context, userID, stripeCustomerID string, tier SubscriptionTier, expiry int64) error
	GetByStripeCustomerID(ctx context.Context, customerID string) (*User, error)
}

// Ensure implementations satisfy the interface
var _ UserService = (*Service)(nil)
var _ UserService = (*MemoryService)(nil)
