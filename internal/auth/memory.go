package auth

import (
	"context"
	"sync"
	"time"
)

// MemoryService is an in-memory token store for local development
type MemoryService struct {
	tokens map[string]*Token
	mu     sync.RWMutex
}

func NewMemoryService() *MemoryService {
	return &MemoryService{
		tokens: make(map[string]*Token),
	}
}

func (s *MemoryService) CreateMagicLinkToken(ctx context.Context, email string) (string, error) {
	tokenStr, err := generateToken()
	if err != nil {
		return "", err
	}

	now := time.Now()
	token := &Token{
		Token:     tokenStr,
		Email:     email,
		Type:      TokenTypeMagicLink,
		TTL:       now.Add(MagicLinkTTL).Unix(),
		CreatedAt: now.Unix(),
	}

	s.mu.Lock()
	s.tokens[tokenStr] = token
	s.mu.Unlock()

	return tokenStr, nil
}

func (s *MemoryService) VerifyMagicLinkToken(ctx context.Context, tokenStr string) (*Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	token, ok := s.tokens[tokenStr]
	if !ok {
		return nil, ErrTokenNotFound
	}

	if token.Type != TokenTypeMagicLink {
		return nil, ErrTokenNotFound
	}

	if token.TTL < time.Now().Unix() {
		delete(s.tokens, tokenStr)
		return nil, ErrTokenExpired
	}

	// Delete the magic link token (one-time use)
	delete(s.tokens, tokenStr)

	return token, nil
}

func (s *MemoryService) CreateSessionToken(ctx context.Context, userID, email string) (string, error) {
	tokenStr, err := generateToken()
	if err != nil {
		return "", err
	}

	now := time.Now()
	token := &Token{
		Token:     tokenStr,
		Email:     email,
		UserID:    userID,
		Type:      TokenTypeSession,
		TTL:       now.Add(SessionTTL).Unix(),
		CreatedAt: now.Unix(),
	}

	s.mu.Lock()
	s.tokens[tokenStr] = token
	s.mu.Unlock()

	return tokenStr, nil
}

func (s *MemoryService) VerifySessionToken(ctx context.Context, tokenStr string) (*Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, ok := s.tokens[tokenStr]
	if !ok {
		return nil, ErrTokenNotFound
	}

	if token.Type != TokenTypeSession {
		return nil, ErrTokenNotFound
	}

	if token.TTL < time.Now().Unix() {
		return nil, ErrTokenExpired
	}

	return token, nil
}

func (s *MemoryService) DeleteSessionToken(ctx context.Context, tokenStr string) error {
	s.mu.Lock()
	delete(s.tokens, tokenStr)
	s.mu.Unlock()
	return nil
}
