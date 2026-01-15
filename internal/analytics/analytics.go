package analytics

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Event types
const (
	EventSearch         = "search"
	EventRateLimitHit   = "ratelimit_hit"
	EventAffiliateClick = "affiliate_click"
	EventSignup         = "signup"
	EventUpgrade        = "upgrade"
	EventChurn          = "churn"
)

// Service handles analytics tracking and querying
type Service struct {
	db        *dynamodb.Client
	tableName string
}

// DailyCounter represents a daily count for a metric
type DailyCounter struct {
	MetricKey string `dynamodbav:"pk"`
	Date      string `dynamodbav:"sk"`
	Count     int64  `dynamodbav:"count"`
	UpdatedAt int64  `dynamodbav:"updated_at"`
}

// Event represents a tracked event
type Event struct {
	MetricKey string `dynamodbav:"pk"`
	Timestamp string `dynamodbav:"sk"`
	EventType string `dynamodbav:"event_type"`
	UserID    string `dynamodbav:"user_id,omitempty"`
	Email     string `dynamodbav:"email,omitempty"`
	IPAddress string `dynamodbav:"ip_address,omitempty"`
	Metadata  string `dynamodbav:"metadata,omitempty"`
	TTL       int64  `dynamodbav:"ttl,omitempty"`
}

// NewService creates a new analytics service
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

// TrackSearch records a search event
func (s *Service) TrackSearch(ctx context.Context, userID, email, ipAddress, description, tldStyle string) {
	go func() {
		s.incrementDailyCounter(ctx, "searches")
		if userID != "" {
			s.incrementDailyCounter(ctx, fmt.Sprintf("user_searches#%s", userID))
		}
		s.recordEvent(ctx, EventSearch, userID, email, ipAddress, fmt.Sprintf("%s|%s", tldStyle, truncate(description, 100)))
	}()
}

// TrackRateLimitHit records when a user hits rate limit
func (s *Service) TrackRateLimitHit(ctx context.Context, ipAddress, reason string, isPro bool) {
	go func() {
		s.incrementDailyCounter(ctx, "ratelimit_hits")
		metadata := fmt.Sprintf("%s|isPro=%v", reason, isPro)
		s.recordEvent(ctx, EventRateLimitHit, "", "", ipAddress, metadata)
	}()
}

// TrackAffiliateClick records a click to Namecheap
func (s *Service) TrackAffiliateClick(ctx context.Context, userID, domain string) {
	go func() {
		s.incrementDailyCounter(ctx, "affiliate_clicks")
		s.recordEvent(ctx, EventAffiliateClick, userID, "", "", domain)
	}()
}

// TrackSignup records a new user signup
func (s *Service) TrackSignup(ctx context.Context, userID, email string) {
	go func() {
		s.incrementDailyCounter(ctx, "signups")
		s.recordEvent(ctx, EventSignup, userID, email, "", "")
	}()
}

// TrackUpgrade records a Pro upgrade
func (s *Service) TrackUpgrade(ctx context.Context, userID, email string) {
	go func() {
		s.incrementDailyCounter(ctx, "upgrades")
		s.recordEvent(ctx, EventUpgrade, userID, email, "", "")
	}()
}

// TrackChurn records a subscription cancellation
func (s *Service) TrackChurn(ctx context.Context, userID, email string) {
	go func() {
		s.incrementDailyCounter(ctx, "churns")
		s.recordEvent(ctx, EventChurn, userID, email, "", "")
	}()
}

// GetDailyCount gets count for a metric on a specific date
func (s *Service) GetDailyCount(ctx context.Context, metric, date string) (int64, error) {
	result, err := s.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("counter#%s", metric)},
			"sk": &types.AttributeValueMemberS{Value: date},
		},
	})
	if err != nil {
		return 0, err
	}
	if result.Item == nil {
		return 0, nil
	}

	var counter DailyCounter
	if err := attributevalue.UnmarshalMap(result.Item, &counter); err != nil {
		return 0, err
	}
	return counter.Count, nil
}

// GetCountRange gets sum of counts for a metric over a date range
func (s *Service) GetCountRange(ctx context.Context, metric, startDate, endDate string) (int64, error) {
	result, err := s.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("pk = :pk AND sk BETWEEN :start AND :end"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":    &types.AttributeValueMemberS{Value: fmt.Sprintf("counter#%s", metric)},
			":start": &types.AttributeValueMemberS{Value: startDate},
			":end":   &types.AttributeValueMemberS{Value: endDate},
		},
	})
	if err != nil {
		return 0, err
	}

	var total int64
	for _, item := range result.Items {
		var counter DailyCounter
		if err := attributevalue.UnmarshalMap(item, &counter); err != nil {
			continue
		}
		total += counter.Count
	}
	return total, nil
}

// GetTotalCount gets the all-time total for a metric
func (s *Service) GetTotalCount(ctx context.Context, metric string) (int64, error) {
	result, err := s.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("total#%s", metric)},
			"sk": &types.AttributeValueMemberS{Value: "total"},
		},
	})
	if err != nil {
		return 0, err
	}
	if result.Item == nil {
		return 0, nil
	}

	var counter DailyCounter
	if err := attributevalue.UnmarshalMap(result.Item, &counter); err != nil {
		return 0, err
	}
	return counter.Count, nil
}

// GetRecentEvents gets recent events of a specific type
func (s *Service) GetRecentEvents(ctx context.Context, eventType string, limit int) ([]Event, error) {
	result, err := s.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("event#%s", eventType)},
		},
		ScanIndexForward: aws.Bool(false), // newest first
		Limit:            aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, err
	}

	var events []Event
	for _, item := range result.Items {
		var event Event
		if err := attributevalue.UnmarshalMap(item, &event); err != nil {
			continue
		}
		events = append(events, event)
	}
	return events, nil
}

// GetDailyTrend gets daily counts for a metric over the past N days
func (s *Service) GetDailyTrend(ctx context.Context, metric string, days int) ([]DailyCounter, error) {
	now := time.Now()
	endDate := now.Format("2006-01-02")
	startDate := now.AddDate(0, 0, -days+1).Format("2006-01-02")

	result, err := s.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("pk = :pk AND sk BETWEEN :start AND :end"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":    &types.AttributeValueMemberS{Value: fmt.Sprintf("counter#%s", metric)},
			":start": &types.AttributeValueMemberS{Value: startDate},
			":end":   &types.AttributeValueMemberS{Value: endDate},
		},
	})
	if err != nil {
		return nil, err
	}

	var counters []DailyCounter
	for _, item := range result.Items {
		var counter DailyCounter
		if err := attributevalue.UnmarshalMap(item, &counter); err != nil {
			continue
		}
		counters = append(counters, counter)
	}
	return counters, nil
}

func (s *Service) incrementDailyCounter(ctx context.Context, metric string) {
	today := time.Now().Format("2006-01-02")

	// Increment daily counter
	_, err := s.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("counter#%s", metric)},
			"sk": &types.AttributeValueMemberS{Value: today},
		},
		UpdateExpression: aws.String("ADD #count :inc SET updated_at = :now"),
		ExpressionAttributeNames: map[string]string{
			"#count": "count",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inc": &types.AttributeValueMemberN{Value: "1"},
			":now": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Unix())},
		},
	})
	if err != nil {
		log.Printf("[ANALYTICS] Failed to increment daily counter %s: %v", metric, err)
	}

	// Also increment total counter
	_, err = s.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("total#%s", metric)},
			"sk": &types.AttributeValueMemberS{Value: "total"},
		},
		UpdateExpression: aws.String("ADD #count :inc SET updated_at = :now"),
		ExpressionAttributeNames: map[string]string{
			"#count": "count",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inc": &types.AttributeValueMemberN{Value: "1"},
			":now": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Unix())},
		},
	})
	if err != nil {
		log.Printf("[ANALYTICS] Failed to increment total counter %s: %v", metric, err)
	}
}

func (s *Service) recordEvent(ctx context.Context, eventType, userID, email, ipAddress, metadata string) {
	now := time.Now()
	// TTL: keep events for 90 days
	ttl := now.Add(90 * 24 * time.Hour).Unix()

	event := Event{
		MetricKey: fmt.Sprintf("event#%s", eventType),
		Timestamp: now.Format(time.RFC3339Nano),
		EventType: eventType,
		UserID:    userID,
		Email:     email,
		IPAddress: ipAddress,
		Metadata:  metadata,
		TTL:       ttl,
	}

	av, err := attributevalue.MarshalMap(event)
	if err != nil {
		log.Printf("[ANALYTICS] Failed to marshal event: %v", err)
		return
	}

	_, err = s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      av,
	})
	if err != nil {
		log.Printf("[ANALYTICS] Failed to record event %s: %v", eventType, err)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// MemoryService is an in-memory analytics service for local development
type MemoryService struct {
	mu       sync.RWMutex
	counters map[string]int64
	events   map[string][]Event
}

// NewMemoryService creates a new in-memory analytics service
func NewMemoryService() *MemoryService {
	return &MemoryService{
		counters: make(map[string]int64),
		events:   make(map[string][]Event),
	}
}

func (m *MemoryService) TrackSearch(ctx context.Context, userID, email, ipAddress, description, tldStyle string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	m.counters[fmt.Sprintf("searches#%s", today)]++
	m.counters["searches#total"]++
	if userID != "" {
		m.counters[fmt.Sprintf("user_searches#%s#%s", userID, today)]++
	}
}

func (m *MemoryService) TrackRateLimitHit(ctx context.Context, ipAddress, reason string, isPro bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	m.counters[fmt.Sprintf("ratelimit_hits#%s", today)]++
	m.counters["ratelimit_hits#total"]++
}

func (m *MemoryService) TrackAffiliateClick(ctx context.Context, userID, domain string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	m.counters[fmt.Sprintf("affiliate_clicks#%s", today)]++
	m.counters["affiliate_clicks#total"]++
}

func (m *MemoryService) TrackSignup(ctx context.Context, userID, email string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	m.counters[fmt.Sprintf("signups#%s", today)]++
	m.counters["signups#total"]++
}

func (m *MemoryService) TrackUpgrade(ctx context.Context, userID, email string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	m.counters[fmt.Sprintf("upgrades#%s", today)]++
	m.counters["upgrades#total"]++
	m.events["upgrade"] = append(m.events["upgrade"], Event{
		UserID:    userID,
		Email:     email,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (m *MemoryService) TrackChurn(ctx context.Context, userID, email string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	m.counters[fmt.Sprintf("churns#%s", today)]++
	m.counters["churns#total"]++
}

func (m *MemoryService) GetDailyCount(ctx context.Context, metric, date string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.counters[fmt.Sprintf("%s#%s", metric, date)], nil
}

func (m *MemoryService) GetCountRange(ctx context.Context, metric, startDate, endDate string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Simplified: just return total for the range
	var total int64
	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		total += m.counters[fmt.Sprintf("%s#%s", metric, d.Format("2006-01-02"))]
	}
	return total, nil
}

func (m *MemoryService) GetTotalCount(ctx context.Context, metric string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.counters[fmt.Sprintf("%s#total", metric)], nil
}

func (m *MemoryService) GetRecentEvents(ctx context.Context, eventType string, limit int) ([]Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	events := m.events[eventType]
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	return events, nil
}

func (m *MemoryService) GetDailyTrend(ctx context.Context, metric string, days int) ([]DailyCounter, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var counters []DailyCounter
	now := time.Now()
	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		counters = append(counters, DailyCounter{
			Date:  date,
			Count: m.counters[fmt.Sprintf("%s#%s", metric, date)],
		})
	}
	return counters, nil
}

// Analytics is the interface for analytics tracking
type Analytics interface {
	TrackSearch(ctx context.Context, userID, email, ipAddress, description, tldStyle string)
	TrackRateLimitHit(ctx context.Context, ipAddress, reason string, isPro bool)
	TrackAffiliateClick(ctx context.Context, userID, domain string)
	TrackSignup(ctx context.Context, userID, email string)
	TrackUpgrade(ctx context.Context, userID, email string)
	TrackChurn(ctx context.Context, userID, email string)
	GetDailyCount(ctx context.Context, metric, date string) (int64, error)
	GetCountRange(ctx context.Context, metric, startDate, endDate string) (int64, error)
	GetTotalCount(ctx context.Context, metric string) (int64, error)
	GetRecentEvents(ctx context.Context, eventType string, limit int) ([]Event, error)
	GetDailyTrend(ctx context.Context, metric string, days int) ([]DailyCounter, error)
}

// Ensure implementations satisfy the interface
var _ Analytics = (*Service)(nil)
var _ Analytics = (*MemoryService)(nil)

// Global analytics instance
var globalAnalytics Analytics

// Init initializes the global analytics instance
func Init() Analytics {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		svc, err := NewService("colettedn-analytics")
		if err != nil {
			log.Printf("[ANALYTICS] Failed to initialize DynamoDB service: %v", err)
			globalAnalytics = NewMemoryService()
		} else {
			globalAnalytics = svc
		}
	} else {
		log.Println("[DEV] Using in-memory analytics")
		globalAnalytics = NewMemoryService()
	}
	return globalAnalytics
}

// Get returns the global analytics instance
func Get() Analytics {
	if globalAnalytics == nil {
		return Init()
	}
	return globalAnalytics
}
