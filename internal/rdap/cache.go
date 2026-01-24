package rdap

import (
	"sync"
	"time"
)

// Cache provides in-memory caching for RDAP lookups
// RDAP data changes infrequently, so we use a longer TTL (7 days)
type Cache struct {
	entries map[string]*cacheEntry
	ttl     time.Duration
	mu      sync.RWMutex
}

type cacheEntry struct {
	Info      *DomainInfo
	FetchedAt time.Time
}

// CachedDomainInfo extends DomainInfo with cache metadata
type CachedDomainInfo struct {
	*DomainInfo
	FromCache bool  `json:"fromCache"`
	CachedAt  int64 `json:"cachedAt,omitempty"` // Unix timestamp
}

// NewCache creates a new RDAP cache
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// Get retrieves a cached RDAP result
func (c *Cache) Get(domain string) (*CachedDomainInfo, bool) {
	c.mu.RLock()
	entry, ok := c.entries[domain]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	// Check TTL
	if time.Since(entry.FetchedAt) > c.ttl {
		c.mu.Lock()
		delete(c.entries, domain)
		c.mu.Unlock()
		return nil, false
	}

	return &CachedDomainInfo{
		DomainInfo: entry.Info,
		FromCache:  true,
		CachedAt:   entry.FetchedAt.Unix(),
	}, true
}

// Set stores an RDAP result in cache
func (c *Cache) Set(domain string, info *DomainInfo) {
	c.mu.Lock()
	c.entries[domain] = &cacheEntry{
		Info:      info,
		FetchedAt: time.Now(),
	}
	c.mu.Unlock()
}

// GetMany retrieves multiple cached results, returning cached and uncached lists
func (c *Cache) GetMany(domains []string) (cached map[string]*CachedDomainInfo, uncached []string) {
	cached = make(map[string]*CachedDomainInfo)

	for _, d := range domains {
		if info, ok := c.Get(d); ok {
			cached[d] = info
		} else {
			uncached = append(uncached, d)
		}
	}

	return cached, uncached
}

// Cleanup removes expired entries (call periodically)
func (c *Cache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for domain, entry := range c.entries {
		if now.Sub(entry.FetchedAt) > c.ttl {
			delete(c.entries, domain)
		}
	}
}

// Size returns the number of cached entries
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
