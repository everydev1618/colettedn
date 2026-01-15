package cache

import "time"

// Cacher is the interface for domain availability caching
type Cacher interface {
	Get(domain string) (*CachedResult, bool)
	GetMany(domains []string) (map[string]*CachedResult, []string)
	Set(domain string, available, isPremium bool, price *float64) error
	Close() error
}

// CachedResult represents a cached domain availability result
type CachedResult struct {
	Domain    string
	Available bool
	IsPremium bool
	Price     *float64
	CheckedAt time.Time
}
