package favorites

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

type Favorite struct {
	UserID    string `dynamodbav:"user_id" json:"userId"`
	Domain    string `dynamodbav:"domain" json:"domain"`
	CreatedAt int64  `dynamodbav:"created_at" json:"createdAt"`
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

func (s *Service) Add(ctx context.Context, userID, domain string) (*Favorite, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	fav := &Favorite{
		UserID:    userID,
		Domain:    domain,
		CreatedAt: time.Now().Unix(),
	}

	av, err := attributevalue.MarshalMap(fav)
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

	return fav, nil
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

func (s *Service) List(ctx context.Context, userID string) ([]Favorite, error) {
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

	var favorites []Favorite
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &favorites); err != nil {
		return nil, err
	}

	return favorites, nil
}

func (s *Service) IsFavorite(ctx context.Context, userID, domain string) (bool, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	result, err := s.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
			"domain":  &types.AttributeValueMemberS{Value: domain},
		},
	})
	if err != nil {
		return false, err
	}

	return result.Item != nil, nil
}

func (s *Service) GetFavoritesMap(ctx context.Context, userID string, domains []string) (map[string]bool, error) {
	favorites, err := s.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	favMap := make(map[string]bool)
	for _, f := range favorites {
		favMap[f.Domain] = true
	}

	return favMap, nil
}
