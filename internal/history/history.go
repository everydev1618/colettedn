package history

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// SearchResult represents a domain result in history
type SearchResult struct {
	Name      string   `json:"name"`
	Available *bool    `json:"available,omitempty"`
	IsPremium *bool    `json:"isPremium,omitempty"`
	Price     *float64 `json:"price,omitempty"`
}

// SearchHistory represents a saved search
type SearchHistory struct {
	UserID      string                    `json:"userId" dynamodbav:"user_id"`
	SearchedAt  int64                     `json:"searchedAt" dynamodbav:"searched_at"`
	Description string                    `json:"description" dynamodbav:"description"`
	TLDStyle    string                    `json:"tldStyle" dynamodbav:"tld_style"`
	Categories  map[string][]SearchResult `json:"categories" dynamodbav:"categories"`
	DomainCount int                       `json:"domainCount" dynamodbav:"domain_count"`
}

// dynamoItem is the DynamoDB representation
type dynamoItem struct {
	UserID      string `dynamodbav:"user_id"`
	SearchedAt  int64  `dynamodbav:"searched_at"`
	Description string `dynamodbav:"description"`
	TLDStyle    string `dynamodbav:"tld_style"`
	Categories  string `dynamodbav:"categories"` // JSON string
	DomainCount int    `dynamodbav:"domain_count"`
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

func (s *Service) Save(ctx context.Context, userID, description, tldStyle string, categories map[string][]SearchResult) (*SearchHistory, error) {
	// Count total domains
	domainCount := 0
	for _, results := range categories {
		domainCount += len(results)
	}

	// Marshal categories to JSON
	categoriesJSON, err := json.Marshal(categories)
	if err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	item := dynamoItem{
		UserID:      userID,
		SearchedAt:  now,
		Description: description,
		TLDStyle:    tldStyle,
		Categories:  string(categoriesJSON),
		DomainCount: domainCount,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return nil, err
	}

	_, err = s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      av,
	})
	if err != nil {
		return nil, err
	}

	return &SearchHistory{
		UserID:      userID,
		SearchedAt:  now,
		Description: description,
		TLDStyle:    tldStyle,
		Categories:  categories,
		DomainCount: domainCount,
	}, nil
}

func (s *Service) List(ctx context.Context, userID string, limit int) ([]SearchHistory, error) {
	if limit <= 0 {
		limit = 20
	}

	result, err := s.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
		ScanIndexForward: aws.Bool(false), // newest first
		Limit:            aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, err
	}

	var items []dynamoItem
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &items); err != nil {
		return nil, err
	}

	// Convert to SearchHistory
	histories := make([]SearchHistory, len(items))
	for i, item := range items {
		var categories map[string][]SearchResult
		if err := json.Unmarshal([]byte(item.Categories), &categories); err != nil {
			categories = make(map[string][]SearchResult)
		}

		histories[i] = SearchHistory{
			UserID:      item.UserID,
			SearchedAt:  item.SearchedAt,
			Description: item.Description,
			TLDStyle:    item.TLDStyle,
			Categories:  categories,
			DomainCount: item.DomainCount,
		}
	}

	return histories, nil
}

func (s *Service) Delete(ctx context.Context, userID string, searchedAt int64) error {
	_, err := s.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"user_id":     &types.AttributeValueMemberS{Value: userID},
			"searched_at": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", searchedAt)},
		},
	})
	return err
}
