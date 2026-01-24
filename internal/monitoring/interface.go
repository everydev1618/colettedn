package monitoring

import "context"

// NotificationThreshold represents which expiry notification was last sent
type NotificationThreshold int

const (
	ThresholdNone    NotificationThreshold = 0
	Threshold30Days  NotificationThreshold = 30
	Threshold7Days   NotificationThreshold = 7
	Threshold1Day    NotificationThreshold = 1
	ThresholdExpired NotificationThreshold = -1
)

// MonitoredDomain represents a domain being monitored for expiration
type MonitoredDomain struct {
	UserID          string  `dynamodbav:"user_id" json:"userId"`
	Domain          string  `dynamodbav:"domain" json:"domain"`
	ExpirationDate  *string `dynamodbav:"expiration_date,omitempty" json:"expirationDate,omitempty"`
	DaysUntilExpiry *int    `dynamodbav:"days_until_expiry,omitempty" json:"daysUntilExpiry,omitempty"`
	Registrar       string  `dynamodbav:"registrar,omitempty" json:"registrar,omitempty"`
	CreatedAt       int64   `dynamodbav:"created_at" json:"createdAt"`
	LastCheckedAt   int64   `dynamodbav:"last_checked_at" json:"lastCheckedAt"`
	// Notification tracking
	LastNotifiedAt          int64                 `dynamodbav:"last_notified_at,omitempty" json:"lastNotifiedAt,omitempty"`
	LastNotificationThreshold NotificationThreshold `dynamodbav:"last_notification_threshold,omitempty" json:"lastNotificationThreshold,omitempty"`
}

// MonitoringService defines the interface for domain monitoring operations
type MonitoringService interface {
	Add(ctx context.Context, userID string, domain *MonitoredDomain) (*MonitoredDomain, error)
	Remove(ctx context.Context, userID, domain string) error
	List(ctx context.Context, userID string) ([]MonitoredDomain, error)
	ListAll(ctx context.Context) ([]MonitoredDomain, error) // For notification scanning
	Get(ctx context.Context, userID, domain string) (*MonitoredDomain, error)
	Update(ctx context.Context, userID string, domain *MonitoredDomain) error
	GetTotalCount(ctx context.Context) (int64, error)
}

// Ensure implementations satisfy the interface
var _ MonitoringService = (*Service)(nil)
var _ MonitoringService = (*MemoryService)(nil)
