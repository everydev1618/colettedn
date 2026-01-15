package owned

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

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

func (s *Service) Add(ctx context.Context, userID, domain string, acquisitionType AcquisitionType) (*OwnedDomain, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	owned := &OwnedDomain{
		UserID:          userID,
		Domain:          domain,
		AcquisitionType: acquisitionType,
		CreatedAt:       time.Now().Unix(),
	}

	av, err := attributevalue.MarshalMap(owned)
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

	return owned, nil
}

func (s *Service) Remove(ctx context.Context, userID, domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))

	_, err := s.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
			"domain":  &types.AttributeValueMemberS{Value: domain},
		},
	})
	return err
}

func (s *Service) List(ctx context.Context, userID string) ([]OwnedDomain, error) {
	result, err := s.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
		ScanIndexForward: aws.Bool(false), // newest first
	})
	if err != nil {
		return nil, err
	}

	var domains []OwnedDomain
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &domains); err != nil {
		return nil, err
	}

	return domains, nil
}

func (s *Service) IsOwned(ctx context.Context, userID, domain string) (*OwnedDomain, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	result, err := s.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
			"domain":  &types.AttributeValueMemberS{Value: domain},
		},
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, nil
	}

	var owned OwnedDomain
	if err := attributevalue.UnmarshalMap(result.Item, &owned); err != nil {
		return nil, err
	}

	return &owned, nil
}

func (s *Service) GetTotalCount(ctx context.Context) (int64, error) {
	result, err := s.db.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(s.tableName),
		Select:    types.SelectCount,
	})
	if err != nil {
		return 0, err
	}
	return int64(result.Count), nil
}

func (s *Service) GetCountByType(ctx context.Context) (map[AcquisitionType]int64, error) {
	result, err := s.db.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(s.tableName),
	})
	if err != nil {
		return nil, err
	}

	var domains []OwnedDomain
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &domains); err != nil {
		return nil, err
	}

	counts := map[AcquisitionType]int64{
		AcquisitionPreviouslyOwned: 0,
		AcquisitionFoundViaColette: 0,
	}

	for _, d := range domains {
		counts[d.AcquisitionType]++
	}

	return counts, nil
}

func (s *Service) ListRecent(ctx context.Context, limit int) ([]OwnedDomain, error) {
	result, err := s.db.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(s.tableName),
	})
	if err != nil {
		return nil, err
	}

	var domains []OwnedDomain
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &domains); err != nil {
		return nil, err
	}

	// Sort by created_at descending
	for i := 0; i < len(domains)-1; i++ {
		for j := i + 1; j < len(domains); j++ {
			if domains[j].CreatedAt > domains[i].CreatedAt {
				domains[i], domains[j] = domains[j], domains[i]
			}
		}
	}

	if len(domains) > limit {
		domains = domains[:limit]
	}

	return domains, nil
}
