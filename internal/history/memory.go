package history

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryService provides an in-memory implementation for local development
type MemoryService struct {
	mu      sync.RWMutex
	history map[string][]SearchHistory // userID -> searches
}

// NewMemoryService creates a new in-memory history service
func NewMemoryService() *MemoryService {
	return &MemoryService{
		history: make(map[string][]SearchHistory),
	}
}

func (s *MemoryService) Save(ctx context.Context, userID, description, tldStyle string, categories map[string][]SearchResult) (*SearchHistory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	domainCount := 0
	for _, results := range categories {
		domainCount += len(results)
	}

	h := SearchHistory{
		UserID:      userID,
		SearchedAt:  time.Now().UnixMilli(),
		Description: description,
		TLDStyle:    tldStyle,
		Categories:  categories,
		DomainCount: domainCount,
	}

	s.history[userID] = append(s.history[userID], h)
	return &h, nil
}

func (s *MemoryService) List(ctx context.Context, userID string, limit int) ([]SearchHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	histories := s.history[userID]
	if histories == nil {
		return []SearchHistory{}, nil
	}

	// Sort by searchedAt descending (newest first)
	sorted := make([]SearchHistory, len(histories))
	copy(sorted, histories)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].SearchedAt > sorted[j].SearchedAt
	})

	// Apply limit
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	return sorted, nil
}

func (s *MemoryService) Delete(ctx context.Context, userID string, searchedAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	histories := s.history[userID]
	for i, h := range histories {
		if h.SearchedAt == searchedAt {
			s.history[userID] = append(histories[:i], histories[i+1:]...)
			break
		}
	}
	return nil
}
