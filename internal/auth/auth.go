package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	MagicLinkTTL = 15 * time.Minute
	SessionTTL   = 30 * 24 * time.Hour // 30 days
)

var (
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenExpired  = errors.New("token expired")
)

type TokenType string

const (
	TokenTypeMagicLink TokenType = "magic_link"
	TokenTypeSession   TokenType = "session"
)

type Token struct {
	Token     string    `dynamodbav:"token"`
	Email     string    `dynamodbav:"email"`
	UserID    string    `dynamodbav:"user_id,omitempty"`
	Type      TokenType `dynamodbav:"type"`
	TTL       int64     `dynamodbav:"ttl"`
	CreatedAt int64     `dynamodbav:"created_at"`
}

type Service struct {
	db        *dynamodb.Client
	tableName string
}

func NewService(tableName string) (*Service, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	return &Service{
		db:        dynamodb.NewFromConfig(cfg),
		tableName: tableName,
	}, nil
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *Service) CreateMagicLinkToken(ctx context.Context, email string) (string, error) {
	tokenStr, err := generateToken()
	if err != nil {
		return "", err
	}

	now := time.Now()
	token := Token{
		Token:     tokenStr,
		Email:     email,
		Type:      TokenTypeMagicLink,
		TTL:       now.Add(MagicLinkTTL).Unix(),
		CreatedAt: now.Unix(),
	}

	av, err := attributevalue.MarshalMap(token)
	if err != nil {
		return "", err
	}

	_, err = s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      av,
	})
	if err != nil {
		return "", err
	}

	return tokenStr, nil
}

func (s *Service) VerifyMagicLinkToken(ctx context.Context, tokenStr string) (*Token, error) {
	result, err := s.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"token": &types.AttributeValueMemberS{Value: tokenStr},
		},
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, ErrTokenNotFound
	}

	var token Token
	if err := attributevalue.UnmarshalMap(result.Item, &token); err != nil {
		return nil, err
	}

	if token.Type != TokenTypeMagicLink {
		return nil, ErrTokenNotFound
	}

	if token.TTL < time.Now().Unix() {
		return nil, ErrTokenExpired
	}

	// Delete the magic link token (one-time use)
	_, _ = s.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"token": &types.AttributeValueMemberS{Value: tokenStr},
		},
	})

	return &token, nil
}

func (s *Service) CreateSessionToken(ctx context.Context, userID, email string) (string, error) {
	tokenStr, err := generateToken()
	if err != nil {
		return "", err
	}

	now := time.Now()
	token := Token{
		Token:     tokenStr,
		Email:     email,
		UserID:    userID,
		Type:      TokenTypeSession,
		TTL:       now.Add(SessionTTL).Unix(),
		CreatedAt: now.Unix(),
	}

	av, err := attributevalue.MarshalMap(token)
	if err != nil {
		return "", err
	}

	_, err = s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      av,
	})
	if err != nil {
		return "", err
	}

	return tokenStr, nil
}

func (s *Service) VerifySessionToken(ctx context.Context, tokenStr string) (*Token, error) {
	result, err := s.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"token": &types.AttributeValueMemberS{Value: tokenStr},
		},
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, ErrTokenNotFound
	}

	var token Token
	if err := attributevalue.UnmarshalMap(result.Item, &token); err != nil {
		return nil, err
	}

	if token.Type != TokenTypeSession {
		return nil, ErrTokenNotFound
	}

	if token.TTL < time.Now().Unix() {
		return nil, ErrTokenExpired
	}

	return &token, nil
}

func (s *Service) DeleteSessionToken(ctx context.Context, tokenStr string) error {
	_, err := s.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"token": &types.AttributeValueMemberS{Value: tokenStr},
		},
	})
	return err
}
