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

func (s *MemoryService) UpdatePreferences(ctx context.Context, userID string, preferredRegistrar string, preferredOtherRegistrar string, theme string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	user.PreferredRegistrar = preferredRegistrar
	user.PreferredOtherRegistrar = preferredOtherRegistrar
	user.Theme = theme
	return nil
}

func (s *MemoryService) UpdateMonitoringNotifications(ctx context.Context, userID string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	user.MonitoringNotifications = &enabled
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

func (s *MemoryService) GetStats(ctx context.Context) (*UserStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &UserStats{}
	for _, user := range s.users {
		stats.TotalUsers++
		if user.SubscriptionTier == TierPro {
			stats.ProUsers++
		}
	}
	stats.FreeUsers = stats.TotalUsers - stats.ProUsers
	stats.MRR = float64(stats.ProUsers) * 29.0 / 12.0

	return stats, nil
}

func (s *MemoryService) ListProUsers(ctx context.Context, limit int) ([]*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var users []*User
	for _, user := range s.users {
		if user.SubscriptionTier == TierPro {
			users = append(users, user)
		}
	}

	// Sort by created_at descending
	for i := 0; i < len(users)-1; i++ {
		for j := 0; j < len(users)-i-1; j++ {
			if users[j].CreatedAt < users[j+1].CreatedAt {
				users[j], users[j+1] = users[j+1], users[j]
			}
		}
	}

	if len(users) > limit {
		users = users[:limit]
	}

	return users, nil
}

func (s *MemoryService) ListRecentUsers(ctx context.Context, limit int) ([]*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var users []*User
	for _, user := range s.users {
		users = append(users, user)
	}

	// Sort by created_at descending
	for i := 0; i < len(users)-1; i++ {
		for j := 0; j < len(users)-i-1; j++ {
			if users[j].CreatedAt < users[j+1].CreatedAt {
				users[j], users[j+1] = users[j+1], users[j]
			}
		}
	}

	if len(users) > limit {
		users = users[:limit]
	}

	return users, nil
}
