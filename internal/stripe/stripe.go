package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidSignature = errors.New("invalid webhook signature")
	ErrEventExpired     = errors.New("webhook event too old")
)

type Client struct {
	secretKey     string
	webhookSecret string
	priceID       string
	httpClient    *http.Client
}

func New(secretKey, webhookSecret, priceID string) *Client {
	return &Client{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		priceID:       priceID,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

type CheckoutSession struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// CreateCheckoutSession creates a Stripe Checkout session for the annual subscription
func (c *Client) CreateCheckoutSession(userID, email, successURL, cancelURL string) (*CheckoutSession, error) {
	data := url.Values{}
	data.Set("mode", "subscription")
	data.Set("success_url", successURL)
	data.Set("cancel_url", cancelURL)
	data.Set("customer_email", email)
	data.Set("client_reference_id", userID)
	data.Set("line_items[0][price]", c.priceID)
	data.Set("line_items[0][quantity]", "1")

	req, err := http.NewRequest("POST", "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.secretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("stripe error: %s", string(body))
	}

	var session CheckoutSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}

	return &session, nil
}

type PortalSession struct {
	URL string `json:"url"`
}

// CreatePortalSession creates a Stripe Customer Portal session
func (c *Client) CreatePortalSession(customerID, returnURL string) (*PortalSession, error) {
	data := url.Values{}
	data.Set("customer", customerID)
	data.Set("return_url", returnURL)

	req, err := http.NewRequest("POST", "https://api.stripe.com/v1/billing_portal/sessions", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.secretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("stripe error: %s", string(body))
	}

	var session PortalSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}

	return &session, nil
}

// WebhookEvent represents a Stripe webhook event
type WebhookEvent struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Data WebhookEventData `json:"data"`
}

type WebhookEventData struct {
	Object json.RawMessage `json:"object"`
}

type Subscription struct {
	ID                 string `json:"id"`
	Customer           string `json:"customer"`
	Status             string `json:"status"`
	CurrentPeriodEnd   int64  `json:"current_period_end"`
	CancelAtPeriodEnd  bool   `json:"cancel_at_period_end"`
	ClientReferenceID  string `json:"client_reference_id,omitempty"`
}

type CheckoutSessionCompleted struct {
	ID                string `json:"id"`
	Customer          string `json:"customer"`
	ClientReferenceID string `json:"client_reference_id"`
	Subscription      string `json:"subscription"`
}

// VerifyWebhookSignature verifies the Stripe webhook signature
func (c *Client) VerifyWebhookSignature(payload []byte, signature string) error {
	parts := strings.Split(signature, ",")
	var timestamp string
	var signatures []string

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}

	if timestamp == "" || len(signatures) == 0 {
		return ErrInvalidSignature
	}

	// Check timestamp is within 5 minutes
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrInvalidSignature
	}
	if time.Now().Unix()-ts > 300 {
		return ErrEventExpired
	}

	// Compute expected signature
	signedPayload := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write([]byte(signedPayload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	// Check if any signature matches
	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expectedSig)) {
			return nil
		}
	}

	return ErrInvalidSignature
}

// ParseWebhookEvent parses a webhook event from the payload
func (c *Client) ParseWebhookEvent(payload []byte) (*WebhookEvent, error) {
	var event WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// GetSubscriptionFromEvent extracts subscription data from event
func (c *Client) GetSubscriptionFromEvent(event *WebhookEvent) (*Subscription, error) {
	var sub Subscription
	if err := json.Unmarshal(event.Data.Object, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetCheckoutSessionFromEvent extracts checkout session data from event
func (c *Client) GetCheckoutSessionFromEvent(event *WebhookEvent) (*CheckoutSessionCompleted, error) {
	var session CheckoutSessionCompleted
	if err := json.Unmarshal(event.Data.Object, &session); err != nil {
		return nil, err
	}
	return &session, nil
}
