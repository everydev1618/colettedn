package favorites

import (
	"context"
	"strings"
	"sync"
	"time"
)

// MemoryService is an in-memory favorites store for local development
type MemoryService struct {
	favorites map[string][]Favorite // by user_id
	mu        sync.RWMutex
}

func NewMemoryService() *MemoryService {
	return &MemoryService{
		favorites: make(map[string][]Favorite),
	}
}

func (s *MemoryService) Add(ctx context.Context, userID, domain string) (*Favorite, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists
	for _, f := range s.favorites[userID] {
		if f.Domain == domain {
			return &f, nil
		}
	}

	fav := Favorite{
		UserID:    userID,
		Domain:    domain,
		CreatedAt: time.Now().Unix(),
	}

	s.favorites[userID] = append(s.favorites[userID], fav)
	return &fav, nil
}

func (s *MemoryService) Remove(ctx context.Context, userID, domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))

	s.mu.Lock()
	defer s.mu.Unlock()

	favs := s.favorites[userID]
	for i, f := range favs {
		if f.Domain == domain {
			s.favorites[userID] = append(favs[:i], favs[i+1:]...)
			break
		}
	}

	return nil
}

func (s *MemoryService) List(ctx context.Context, userID string) ([]Favorite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	favs := s.favorites[userID]
	if favs == nil {
		return []Favorite{}, nil
	}

	// Return a copy in reverse order (newest first)
	result := make([]Favorite, len(favs))
	for i, f := range favs {
		result[len(favs)-1-i] = f
	}
	return result, nil
}

func (s *MemoryService) IsFavorite(ctx context.Context, userID, domain string) (bool, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, f := range s.favorites[userID] {
		if f.Domain == domain {
			return true, nil
		}
	}
	return false, nil
}

func (s *MemoryService) GetFavoritesMap(ctx context.Context, userID string, domains []string) (map[string]bool, error) {
	favorites, err := s.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	favMap := make(map[string]bool)
	for _, f := range favorites {
		favMap[f.Domain] = true
	}

	return favMap, nil
}
