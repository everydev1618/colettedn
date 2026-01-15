package owned

import (
	"context"
	"strings"
	"sync"
	"time"
)

type MemoryService struct {
	domains map[string]map[string]*OwnedDomain // userID -> domain -> OwnedDomain
	mu      sync.RWMutex
}

func NewMemoryService() *MemoryService {
	return &MemoryService{
		domains: make(map[string]map[string]*OwnedDomain),
	}
}

func (s *MemoryService) Add(ctx context.Context, userID, domain string, acquisitionType AcquisitionType) (*OwnedDomain, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	owned := &OwnedDomain{
		UserID:          userID,
		Domain:          domain,
		AcquisitionType: acquisitionType,
		CreatedAt:       time.Now().Unix(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.domains[userID] == nil {
		s.domains[userID] = make(map[string]*OwnedDomain)
	}
	s.domains[userID][domain] = owned

	return owned, nil
}

func (s *MemoryService) Remove(ctx context.Context, userID, domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.domains[userID] != nil {
		delete(s.domains[userID], domain)
	}

	return nil
}

func (s *MemoryService) List(ctx context.Context, userID string) ([]OwnedDomain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []OwnedDomain
	if s.domains[userID] != nil {
		for _, d := range s.domains[userID] {
			result = append(result, *d)
		}
	}

	// Sort by created_at descending
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt > result[i].CreatedAt {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

func (s *MemoryService) IsOwned(ctx context.Context, userID, domain string) (*OwnedDomain, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.domains[userID] != nil {
		if owned, ok := s.domains[userID][domain]; ok {
			return owned, nil
		}
	}

	return nil, nil
}

func (s *MemoryService) GetTotalCount(ctx context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, userDomains := range s.domains {
		count += int64(len(userDomains))
	}

	return count, nil
}

func (s *MemoryService) GetCountByType(ctx context.Context) (map[AcquisitionType]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := map[AcquisitionType]int64{
		AcquisitionPreviouslyOwned: 0,
		AcquisitionFoundViaColette: 0,
	}

	for _, userDomains := range s.domains {
		for _, d := range userDomains {
			counts[d.AcquisitionType]++
		}
	}

	return counts, nil
}

func (s *MemoryService) ListRecent(ctx context.Context, limit int) ([]OwnedDomain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []OwnedDomain
	for _, userDomains := range s.domains {
		for _, d := range userDomains {
			all = append(all, *d)
		}
	}

	// Sort by created_at descending
	for i := 0; i < len(all)-1; i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].CreatedAt > all[i].CreatedAt {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	if len(all) > limit {
		all = all[:limit]
	}

	return all, nil
}
