package notifier

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/everydev1618/colettedn/internal/monitoring"
	"github.com/everydev1618/colettedn/internal/user"
)

// Notifier handles sending expiration notifications
type Notifier struct {
	ses               *ses.Client
	monitoringService monitoring.MonitoringService
	userService       user.UserService
	fromEmail         string
	appURL            string
}

// New creates a new Notifier
func New(monitoringService monitoring.MonitoringService, userService user.UserService, fromEmail, appURL string) (*Notifier, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	return &Notifier{
		ses:               ses.NewFromConfig(cfg),
		monitoringService: monitoringService,
		userService:       userService,
		fromEmail:         fromEmail,
		appURL:            appURL,
	}, nil
}

// DomainNotification represents a domain that needs notification
type DomainNotification struct {
	Domain          *monitoring.MonitoredDomain
	Threshold       monitoring.NotificationThreshold
	ThresholdLabel  string // "30 days", "7 days", "1 day", "expired"
}

// UserNotifications groups notifications by user
type UserNotifications struct {
	User    *user.User
	Domains []DomainNotification
}

// Run scans all monitored domains and sends notifications
func (n *Notifier) Run(ctx context.Context) error {
	log.Println("[NOTIFIER] Starting notification scan...")

	// Get all monitored domains
	domains, err := n.monitoringService.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to list domains: %w", err)
	}

	log.Printf("[NOTIFIER] Found %d monitored domains", len(domains))

	if len(domains) == 0 {
		return nil
	}

	// Group domains by user and check thresholds
	userNotifications := make(map[string]*UserNotifications)

	for i := range domains {
		domain := &domains[i]

		// Check if notification is needed
		notification := n.checkNotificationNeeded(domain)
		if notification == nil {
			continue
		}

		// Group by user
		if userNotifications[domain.UserID] == nil {
			userNotifications[domain.UserID] = &UserNotifications{
				Domains: []DomainNotification{},
			}
		}
		userNotifications[domain.UserID].Domains = append(
			userNotifications[domain.UserID].Domains,
			*notification,
		)
	}

	log.Printf("[NOTIFIER] %d users have domains needing notification", len(userNotifications))

	// Fetch user info and send emails
	for userID, notifications := range userNotifications {
		// Get user
		u, err := n.userService.GetByID(ctx, userID)
		if err != nil {
			log.Printf("[NOTIFIER] Failed to get user %s: %v", userID, err)
			continue
		}

		// Check if user has notifications enabled
		if !u.MonitoringNotificationsEnabled() {
			log.Printf("[NOTIFIER] User %s has notifications disabled, skipping", userID)
			continue
		}

		// Only Pro users should have monitoring domains, but double-check
		if u.SubscriptionTier != user.TierPro {
			log.Printf("[NOTIFIER] User %s is not Pro, skipping", userID)
			continue
		}

		notifications.User = u

		// Send email
		if err := n.sendNotificationEmail(ctx, notifications); err != nil {
			log.Printf("[NOTIFIER] Failed to send email to %s: %v", u.Email, err)
			continue
		}

		// Update notification status for each domain
		for _, dn := range notifications.Domains {
			dn.Domain.LastNotifiedAt = time.Now().Unix()
			dn.Domain.LastNotificationThreshold = dn.Threshold
			if err := n.monitoringService.Update(ctx, userID, dn.Domain); err != nil {
				log.Printf("[NOTIFIER] Failed to update domain %s: %v", dn.Domain.Domain, err)
			}
		}

		log.Printf("[NOTIFIER] Sent notification to %s for %d domains", u.Email, len(notifications.Domains))
	}

	log.Println("[NOTIFIER] Notification scan complete")
	return nil
}

// checkNotificationNeeded determines if a domain needs notification
func (n *Notifier) checkNotificationNeeded(domain *monitoring.MonitoredDomain) *DomainNotification {
	if domain.DaysUntilExpiry == nil {
		return nil // No expiry info
	}

	days := *domain.DaysUntilExpiry
	var threshold monitoring.NotificationThreshold
	var label string

	// Determine which threshold applies
	if days <= 0 {
		threshold = monitoring.ThresholdExpired
		label = "expired"
	} else if days <= 1 {
		threshold = monitoring.Threshold1Day
		label = "1 day"
	} else if days <= 7 {
		threshold = monitoring.Threshold7Days
		label = "7 days"
	} else if days <= 30 {
		threshold = monitoring.Threshold30Days
		label = "30 days"
	} else {
		return nil // Not within any notification threshold
	}

	// Check if we already sent this threshold notification
	if domain.LastNotificationThreshold >= threshold && threshold != monitoring.ThresholdExpired {
		// Already notified for this or a more urgent threshold
		// Exception: always notify for expired if it just became expired
		return nil
	}

	// For expired, check if we already notified about expired status
	if threshold == monitoring.ThresholdExpired && domain.LastNotificationThreshold == monitoring.ThresholdExpired {
		// Check if we notified within the last 7 days to avoid spam
		if domain.LastNotifiedAt > 0 {
			lastNotified := time.Unix(domain.LastNotifiedAt, 0)
			if time.Since(lastNotified) < 7*24*time.Hour {
				return nil
			}
		}
	}

	return &DomainNotification{
		Domain:         domain,
		Threshold:      threshold,
		ThresholdLabel: label,
	}
}

// sendNotificationEmail sends a batched notification email to a user
func (n *Notifier) sendNotificationEmail(ctx context.Context, notifications *UserNotifications) error {
	subject := n.getEmailSubject(notifications)
	htmlBody := n.buildEmailHTML(notifications)
	textBody := n.buildEmailText(notifications)

	_, err := n.ses.SendEmail(ctx, &ses.SendEmailInput{
		Source: aws.String(n.fromEmail),
		Destination: &types.Destination{
			ToAddresses: []string{notifications.User.Email},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data:    aws.String(subject),
				Charset: aws.String("UTF-8"),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data:    aws.String(htmlBody),
					Charset: aws.String("UTF-8"),
				},
				Text: &types.Content{
					Data:    aws.String(textBody),
					Charset: aws.String("UTF-8"),
				},
			},
		},
	})

	return err
}

func (n *Notifier) getEmailSubject(notifications *UserNotifications) string {
	// Find the most urgent notification
	mostUrgent := monitoring.Threshold30Days
	for _, dn := range notifications.Domains {
		if dn.Threshold < mostUrgent || dn.Threshold == monitoring.ThresholdExpired {
			mostUrgent = dn.Threshold
		}
	}

	count := len(notifications.Domains)
	domainWord := "domain"
	if count > 1 {
		domainWord = "domains"
	}

	switch mostUrgent {
	case monitoring.ThresholdExpired:
		return fmt.Sprintf("Domain Alert: %d monitored %s may be available!", count, domainWord)
	case monitoring.Threshold1Day:
		return fmt.Sprintf("Urgent: %d monitored %s expires tomorrow!", count, domainWord)
	case monitoring.Threshold7Days:
		return fmt.Sprintf("Reminder: %d monitored %s expires within a week", count, domainWord)
	default:
		return fmt.Sprintf("Heads up: %d monitored %s expires within 30 days", count, domainWord)
	}
}

func (n *Notifier) buildEmailHTML(notifications *UserNotifications) string {
	domainsHTML := ""
	for _, dn := range notifications.Domains {
		urgencyClass := ""
		switch dn.Threshold {
		case monitoring.ThresholdExpired:
			urgencyClass = "color: #d32f2f; font-weight: bold;"
		case monitoring.Threshold1Day:
			urgencyClass = "color: #f57c00; font-weight: bold;"
		case monitoring.Threshold7Days:
			urgencyClass = "color: #ffa000;"
		}

		expiryText := fmt.Sprintf("Expires in %s", dn.ThresholdLabel)
		if dn.Threshold == monitoring.ThresholdExpired {
			expiryText = "Expired - may be available!"
		}

		domainsHTML += fmt.Sprintf(`
            <tr>
                <td style="padding: 12px; border-bottom: 1px solid #eee;">
                    <strong style="font-size: 16px;">%s</strong>
                    %s
                </td>
                <td style="padding: 12px; border-bottom: 1px solid #eee; %s">
                    %s
                </td>
            </tr>
        `, dn.Domain.Domain,
           func() string {
               if dn.Domain.Registrar != "" {
                   return fmt.Sprintf(`<br><span style="font-size: 12px; color: #666;">%s</span>`, dn.Domain.Registrar)
               }
               return ""
           }(),
           urgencyClass,
           expiryText)
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f5f5f5; margin: 0; padding: 40px 20px;">
    <div style="max-width: 560px; margin: 0 auto; background: white; border-radius: 12px; padding: 40px; box-shadow: 0 2px 8px rgba(0,0,0,0.08);">
        <div style="text-align: center; margin-bottom: 32px;">
            <span style="font-size: 24px;">&#10022;</span>
            <span style="font-family: 'Times New Roman', serif; font-size: 28px; font-weight: normal; margin-left: 8px;">Colette</span>
            <span style="font-size: 12px; color: #666; margin-left: 4px;">DN</span>
        </div>

        <h1 style="font-size: 20px; font-weight: 500; text-align: center; margin-bottom: 24px; color: #333;">
            Domain Expiration Alert
        </h1>

        <p style="color: #666; text-align: center; margin-bottom: 24px; line-height: 1.5;">
            The following domains you're monitoring are expiring soon:
        </p>

        <table style="width: 100%%; border-collapse: collapse; margin-bottom: 24px;">
            <thead>
                <tr style="background: #f9f9f9;">
                    <th style="padding: 12px; text-align: left; border-bottom: 2px solid #eee;">Domain</th>
                    <th style="padding: 12px; text-align: left; border-bottom: 2px solid #eee;">Status</th>
                </tr>
            </thead>
            <tbody>
                %s
            </tbody>
        </table>

        <div style="text-align: center; margin-bottom: 24px;">
            <a href="%s" style="display: inline-block; background: #333; color: white; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 500;">
                View Monitoring List
            </a>
        </div>

        <hr style="border: none; border-top: 1px solid #eee; margin: 24px 0;">

        <p style="color: #999; font-size: 12px; text-align: center;">
            You're receiving this because you have domain monitoring enabled.<br>
            <a href="%s" style="color: #666;">Manage notification preferences</a>
        </p>
    </div>
</body>
</html>
`, domainsHTML, n.appURL, n.appURL)
}

func (n *Notifier) buildEmailText(notifications *UserNotifications) string {
	text := "Domain Expiration Alert\n\n"
	text += "The following domains you're monitoring are expiring soon:\n\n"

	for _, dn := range notifications.Domains {
		expiryText := fmt.Sprintf("Expires in %s", dn.ThresholdLabel)
		if dn.Threshold == monitoring.ThresholdExpired {
			expiryText = "Expired - may be available!"
		}
		text += fmt.Sprintf("- %s: %s\n", dn.Domain.Domain, expiryText)
	}

	text += fmt.Sprintf("\nView your monitoring list: %s\n", n.appURL)
	text += "\n---\nYou're receiving this because you have domain monitoring enabled.\n"

	return text
}
