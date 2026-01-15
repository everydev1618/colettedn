package cache

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoCache struct {
	client    *dynamodb.Client
	tableName string
	ttl       time.Duration
}

type dynamoItem struct {
	Domain    string   `dynamodbav:"domain"`
	Available bool     `dynamodbav:"available"`
	IsPremium bool     `dynamodbav:"is_premium"`
	Price     *float64 `dynamodbav:"price,omitempty"`
	TTL       int64    `dynamodbav:"ttl"`
}

func NewDynamo(tableName string, ttl time.Duration) (*DynamoCache, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	client := dynamodb.NewFromConfig(cfg)

	return &DynamoCache{
		client:    client,
		tableName: tableName,
		ttl:       ttl,
	}, nil
}

func (c *DynamoCache) Get(domain string) (*CachedResult, bool) {
	ctx := context.Background()

	result, err := c.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"domain": &types.AttributeValueMemberS{Value: domain},
		},
	})

	if err != nil || result.Item == nil {
		return nil, false
	}

	var item dynamoItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, false
	}

	// Check if expired (TTL is handled by DynamoDB, but double-check)
	if item.TTL < time.Now().Unix() {
		return nil, false
	}

	return &CachedResult{
		Domain:    item.Domain,
		Available: item.Available,
		IsPremium: item.IsPremium,
		Price:     item.Price,
		CheckedAt: time.Unix(item.TTL-int64(c.ttl.Seconds()), 0),
	}, true
}

func (c *DynamoCache) GetMany(domains []string) (map[string]*CachedResult, []string) {
	cached := make(map[string]*CachedResult)
	var uncached []string

	for _, d := range domains {
		if result, ok := c.Get(d); ok {
			cached[d] = result
		} else {
			uncached = append(uncached, d)
		}
	}

	return cached, uncached
}

func (c *DynamoCache) Set(domain string, available, isPremium bool, price *float64) error {
	ctx := context.Background()

	item := dynamoItem{
		Domain:    domain,
		Available: available,
		IsPremium: isPremium,
		Price:     price,
		TTL:       time.Now().Add(c.ttl).Unix(),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}

	_, err = c.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      av,
	})

	return err
}

func (c *DynamoCache) Close() error {
	return nil
}
