package monitoring

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

func (s *Service) Add(ctx context.Context, userID string, domain *MonitoredDomain) (*MonitoredDomain, error) {
	domain.Domain = strings.ToLower(strings.TrimSpace(domain.Domain))
	domain.UserID = userID
	domain.CreatedAt = time.Now().Unix()
	domain.LastCheckedAt = time.Now().Unix()

	av, err := attributevalue.MarshalMap(domain)
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

	return domain, nil
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

func (s *Service) List(ctx context.Context, userID string) ([]MonitoredDomain, error) {
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

	var domains []MonitoredDomain
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &domains); err != nil {
		return nil, err
	}

	return domains, nil
}

func (s *Service) Get(ctx context.Context, userID, domain string) (*MonitoredDomain, error) {
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

	var monitored MonitoredDomain
	if err := attributevalue.UnmarshalMap(result.Item, &monitored); err != nil {
		return nil, err
	}

	return &monitored, nil
}

func (s *Service) Update(ctx context.Context, userID string, domain *MonitoredDomain) error {
	domain.Domain = strings.ToLower(strings.TrimSpace(domain.Domain))
	domain.UserID = userID
	domain.LastCheckedAt = time.Now().Unix()

	av, err := attributevalue.MarshalMap(domain)
	if err != nil {
		return err
	}

	_, err = s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      av,
	})
	return err
}

func (s *Service) ListAll(ctx context.Context) ([]MonitoredDomain, error) {
	var allDomains []MonitoredDomain

	paginator := dynamodb.NewScanPaginator(s.db, &dynamodb.ScanInput{
		TableName: aws.String(s.tableName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		var domains []MonitoredDomain
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &domains); err != nil {
			return nil, err
		}
		allDomains = append(allDomains, domains...)
	}

	return allDomains, nil
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
