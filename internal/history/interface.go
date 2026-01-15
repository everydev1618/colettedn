package history

import "context"

// HistoryService defines the interface for search history operations
type HistoryService interface {
	Save(ctx context.Context, userID, description, tldStyle string, categories map[string][]SearchResult) (*SearchHistory, error)
	List(ctx context.Context, userID string, limit int) ([]SearchHistory, error)
	Delete(ctx context.Context, userID string, searchedAt int64) error
}
