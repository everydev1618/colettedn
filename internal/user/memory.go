package user

import (
	"context"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// MemoryService is an in-memory user store for local development
type MemoryService struct {
	users       map[string]*User // by user_id
	usersByEmail map[string]*User // by email
	mu          sync.RWMutex
}

func NewMemoryService() *MemoryService {
	return &MemoryService{
		users:       make(map[string]*User),
		usersByEmail: make(map[string]*User),
	}
}

func (s *MemoryService) GetByID(ctx context.Context, userID string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[userID]
	if !ok {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *MemoryService) GetByEmail(ctx context.Context, email string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.usersByEmail[email]
	if !ok {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *MemoryService) Create(ctx context.Context, email string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists
	if _, ok := s.usersByEmail[email]; ok {
		return nil, nil // Already exists
	}

	user := &User{
		UserID:           ulid.Make().String(),
		Email:            email,
		SubscriptionTier: TierFree,
		CreatedAt:        time.Now().Unix(),
	}

	s.users[user.UserID] = user
	s.usersByEmail[email] = user

	return user, nil
}

func (s *MemoryService) GetOrCreate(ctx context.Context, email string) (*User, error) {
	user, err := s.GetByEmail(ctx, email)
	if err == nil {
		return user, nil
	}

	return s.Create(ctx, email)
}

func (s *MemoryService) UpdateSubscription(ctx context.Context, userID, stripeCustomerID string, tier SubscriptionTier, expiry int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	user.SubscriptionTier = tier
	user.StripeCustomerID = stripeCustomerID
	user.SubscriptionExpiry = expiry

	return nil
}

func (s *MemoryService) GetByStripeCustomerID(ctx context.Context, customerID string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if user.StripeCustomerID == customerID {
			return user, nil
		}
	}

	return nil, ErrUserNotFound
}
