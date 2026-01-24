package monitoring

import (
	"context"
	"strings"
	"sync"
	"time"
)

type MemoryService struct {
	domains map[string]map[string]*MonitoredDomain // userID -> domain -> MonitoredDomain
	mu      sync.RWMutex
}

func NewMemoryService() *MemoryService {
	return &MemoryService{
		domains: make(map[string]map[string]*MonitoredDomain),
	}
}

func (s *MemoryService) Add(ctx context.Context, userID string, domain *MonitoredDomain) (*MonitoredDomain, error) {
	domain.Domain = strings.ToLower(strings.TrimSpace(domain.Domain))
	domain.UserID = userID
	domain.CreatedAt = time.Now().Unix()
	domain.LastCheckedAt = time.Now().Unix()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.domains[userID] == nil {
		s.domains[userID] = make(map[string]*MonitoredDomain)
	}
	s.domains[userID][domain.Domain] = domain

	return domain, nil
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

func (s *MemoryService) List(ctx context.Context, userID string) ([]MonitoredDomain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []MonitoredDomain
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

func (s *MemoryService) Get(ctx context.Context, userID, domain string) (*MonitoredDomain, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.domains[userID] != nil {
		if monitored, ok := s.domains[userID][domain]; ok {
			return monitored, nil
		}
	}

	return nil, nil
}

func (s *MemoryService) Update(ctx context.Context, userID string, domain *MonitoredDomain) error {
	domain.Domain = strings.ToLower(strings.TrimSpace(domain.Domain))
	domain.UserID = userID
	domain.LastCheckedAt = time.Now().Unix()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.domains[userID] == nil {
		s.domains[userID] = make(map[string]*MonitoredDomain)
	}
	s.domains[userID][domain.Domain] = domain

	return nil
}

func (s *MemoryService) ListAll(ctx context.Context) ([]MonitoredDomain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []MonitoredDomain
	for _, userDomains := range s.domains {
		for _, d := range userDomains {
			result = append(result, *d)
		}
	}

	return result, nil
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
