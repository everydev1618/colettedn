package auth

import "context"

// TokenService defines the interface for token operations
type TokenService interface {
	CreateMagicLinkToken(ctx context.Context, email string) (string, error)
	VerifyMagicLinkToken(ctx context.Context, tokenStr string) (*Token, error)
	CreateSessionToken(ctx context.Context, userID, email string) (string, error)
	VerifySessionToken(ctx context.Context, tokenStr string) (*Token, error)
	DeleteSessionToken(ctx context.Context, tokenStr string) error
}

// Ensure implementations satisfy the interface
var _ TokenService = (*Service)(nil)
var _ TokenService = (*MemoryService)(nil)
