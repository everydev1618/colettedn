package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/oklog/ulid/v2"
)

var ErrUserNotFound = errors.New("user not found")

type SubscriptionTier string

const (
	TierFree SubscriptionTier = "free"
	TierPro  SubscriptionTier = "pro"
)

type User struct {
	UserID              string           `dynamodbav:"user_id" json:"userId"`
	Email               string           `dynamodbav:"email" json:"email"`
	SubscriptionTier    SubscriptionTier `dynamodbav:"subscription_tier" json:"subscriptionTier"`
	StripeCustomerID    string           `dynamodbav:"stripe_customer_id,omitempty" json:"stripeCustomerId,omitempty"`
	SubscriptionExpiry  int64            `dynamodbav:"subscription_expiry,omitempty" json:"subscriptionExpiry,omitempty"`
	PreferredRegistrar  string           `dynamodbav:"preferred_registrar,omitempty" json:"preferredRegistrar,omitempty"`
	CreatedAt           int64            `dynamodbav:"created_at" json:"createdAt"`
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

func (s *Service) GetByID(ctx context.Context, userID string) (*User, error) {
	result, err := s.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, ErrUserNotFound
	}

	var user User
	if err := attributevalue.UnmarshalMap(result.Item, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Service) GetByEmail(ctx context.Context, email string) (*User, error) {
	result, err := s.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String("email-index"),
		KeyConditionExpression: aws.String("email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, ErrUserNotFound
	}

	var user User
	if err := attributevalue.UnmarshalMap(result.Items[0], &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Service) Create(ctx context.Context, email string) (*User, error) {
	user := &User{
		UserID:           ulid.Make().String(),
		Email:            email,
		SubscriptionTier: TierFree,
		CreatedAt:        time.Now().Unix(),
	}

	av, err := attributevalue.MarshalMap(user)
	if err != nil {
		return nil, err
	}

	_, err = s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(user_id)"),
	})
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Service) GetOrCreate(ctx context.Context, email string) (*User, error) {
	user, err := s.GetByEmail(ctx, email)
	if err == nil {
		return user, nil
	}

	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	return s.Create(ctx, email)
}

func (s *Service) UpdateSubscription(ctx context.Context, userID, stripeCustomerID string, tier SubscriptionTier, expiry int64) error {
	_, err := s.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
		UpdateExpression: aws.String("SET subscription_tier = :tier, stripe_customer_id = :cid, subscription_expiry = :exp"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":tier": &types.AttributeValueMemberS{Value: string(tier)},
			":cid":  &types.AttributeValueMemberS{Value: stripeCustomerID},
			":exp":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expiry)},
		},
	})
	return err
}

func (s *Service) UpdatePreferences(ctx context.Context, userID string, preferredRegistrar string) error {
	_, err := s.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
		UpdateExpression: aws.String("SET preferred_registrar = :reg"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":reg": &types.AttributeValueMemberS{Value: preferredRegistrar},
		},
	})
	return err
}

func (s *Service) GetByStripeCustomerID(ctx context.Context, customerID string) (*User, error) {
	result, err := s.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String("stripe-customer-index"),
		KeyConditionExpression: aws.String("stripe_customer_id = :cid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":cid": &types.AttributeValueMemberS{Value: customerID},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, ErrUserNotFound
	}

	var user User
	if err := attributevalue.UnmarshalMap(result.Items[0], &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// GetStats returns aggregate statistics about users
func (s *Service) GetStats(ctx context.Context) (*UserStats, error) {
	stats := &UserStats{}

	// Scan all users to count totals
	// Note: For large user bases, consider using a separate counter table
	paginator := dynamodb.NewScanPaginator(s.db, &dynamodb.ScanInput{
		TableName: aws.String(s.tableName),
		ProjectionExpression: aws.String("subscription_tier"),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, item := range page.Items {
			stats.TotalUsers++
			if tier, ok := item["subscription_tier"]; ok {
				if tierStr, ok := tier.(*types.AttributeValueMemberS); ok && tierStr.Value == string(TierPro) {
					stats.ProUsers++
				}
			}
		}
	}

	stats.FreeUsers = stats.TotalUsers - stats.ProUsers
	// MRR = Pro users * $29/year / 12 months
	stats.MRR = float64(stats.ProUsers) * 29.0 / 12.0

	return stats, nil
}

// ListProUsers returns the most recent Pro subscribers
func (s *Service) ListProUsers(ctx context.Context, limit int) ([]*User, error) {
	result, err := s.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String("tier-created-index"),
		KeyConditionExpression: aws.String("subscription_tier = :tier"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":tier": &types.AttributeValueMemberS{Value: string(TierPro)},
		},
		ScanIndexForward: aws.Bool(false), // newest first
		Limit:            aws.Int32(int32(limit)),
	})
	if err != nil {
		// If index doesn't exist, fall back to scan
		return s.listProUsersFallback(ctx, limit)
	}

	var users []*User
	for _, item := range result.Items {
		var user User
		if err := attributevalue.UnmarshalMap(item, &user); err != nil {
			continue
		}
		users = append(users, &user)
	}

	return users, nil
}

func (s *Service) listProUsersFallback(ctx context.Context, limit int) ([]*User, error) {
	result, err := s.db.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(s.tableName),
		FilterExpression: aws.String("subscription_tier = :tier"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":tier": &types.AttributeValueMemberS{Value: string(TierPro)},
		},
	})
	if err != nil {
		return nil, err
	}

	var users []*User
	for _, item := range result.Items {
		var user User
		if err := attributevalue.UnmarshalMap(item, &user); err != nil {
			continue
		}
		users = append(users, &user)
	}

	// Sort by created_at descending and limit
	// Simple bubble sort for small lists
	for i := 0; i < len(users)-1; i++ {
		for j := 0; j < len(users)-i-1; j++ {
			if users[j].CreatedAt < users[j+1].CreatedAt {
				users[j], users[j+1] = users[j+1], users[j]
			}
		}
	}

	if len(users) > limit {
		users = users[:limit]
	}

	return users, nil
}

// ListRecentUsers returns the most recently created users
func (s *Service) ListRecentUsers(ctx context.Context, limit int) ([]*User, error) {
	// Use scan with limit - not ideal but works for small user bases
	result, err := s.db.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(s.tableName),
	})
	if err != nil {
		return nil, err
	}

	var users []*User
	for _, item := range result.Items {
		var user User
		if err := attributevalue.UnmarshalMap(item, &user); err != nil {
			continue
		}
		users = append(users, &user)
	}

	// Sort by created_at descending
	for i := 0; i < len(users)-1; i++ {
		for j := 0; j < len(users)-i-1; j++ {
			if users[j].CreatedAt < users[j+1].CreatedAt {
				users[j], users[j+1] = users[j+1], users[j]
			}
		}
	}

	if len(users) > limit {
		users = users[:limit]
	}

	return users, nil
}
