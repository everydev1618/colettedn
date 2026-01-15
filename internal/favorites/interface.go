package favorites

import "context"

// FavoritesService defines the interface for favorites operations
type FavoritesService interface {
	Add(ctx context.Context, userID, domain string) (*Favorite, error)
	Remove(ctx context.Context, userID, domain string) error
	List(ctx context.Context, userID string) ([]Favorite, error)
	IsFavorite(ctx context.Context, userID, domain string) (bool, error)
	GetFavoritesMap(ctx context.Context, userID string, domains []string) (map[string]bool, error)
	GetTotalCount(ctx context.Context) (int64, error)
	ListRecent(ctx context.Context, limit int) ([]Favorite, error)
}

// Ensure implementations satisfy the interface
var _ FavoritesService = (*Service)(nil)
var _ FavoritesService = (*MemoryService)(nil)
