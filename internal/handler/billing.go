package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/everydev1618/colettedn/internal/auth"
	"github.com/everydev1618/colettedn/internal/stripe"
	"github.com/everydev1618/colettedn/internal/user"
)

type BillingHandler struct {
	stripeClient *stripe.Client
	userService  user.UserService
	appURL       string
}

func NewBillingHandler(userService user.UserService) (*BillingHandler, error) {
	secretKey := os.Getenv("STRIPE_SECRET_KEY")
	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	priceID := os.Getenv("STRIPE_PRICE_ID")
	appURL := os.Getenv("APP_URL")

	if appURL == "" {
		appURL = "http://localhost:8080"
	}

	// Allow running without Stripe in dev mode
	if secretKey == "" {
		log.Println("[WARN] STRIPE_SECRET_KEY not set, billing endpoints will be disabled")
		return nil, nil
	}

	return &BillingHandler{
		stripeClient: stripe.New(secretKey, webhookSecret, priceID),
		userService:  userService,
		appURL:       appURL,
	}, nil
}

type CheckoutRequest struct {
	// No fields needed - user comes from auth context
}

type CheckoutResponse struct {
	URL   string `json:"url,omitempty"`
	Error string `json:"error,omitempty"`
}

// Checkout creates a Stripe Checkout session for upgrading to Pro
func (h *BillingHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	u := auth.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, CheckoutResponse{Error: "Unauthorized"})
		return
	}

	// Get full user to check if already subscribed
	fullUser, err := h.userService.GetByID(r.Context(), u.UserID)
	if err != nil {
		log.Printf("[BILLING_ERROR] Failed to get user: %v", err)
		writeJSON(w, http.StatusInternalServerError, CheckoutResponse{Error: "Failed to get user"})
		return
	}

	if fullUser.SubscriptionTier == user.TierPro {
		writeJSON(w, http.StatusBadRequest, CheckoutResponse{Error: "Already subscribed to Pro"})
		return
	}

	successURL := h.appURL + "/?upgraded=true"
	cancelURL := h.appURL + "/?cancelled=true"

	session, err := h.stripeClient.CreateCheckoutSession(u.UserID, u.Email, successURL, cancelURL)
	if err != nil {
		log.Printf("[BILLING_ERROR] Failed to create checkout session: %v", err)
		writeJSON(w, http.StatusInternalServerError, CheckoutResponse{Error: "Failed to create checkout session"})
		return
	}

	writeJSON(w, http.StatusOK, CheckoutResponse{URL: session.URL})
}

type PortalResponse struct {
	URL   string `json:"url,omitempty"`
	Error string `json:"error,omitempty"`
}

// Portal creates a Stripe Customer Portal session for managing subscription
func (h *BillingHandler) Portal(w http.ResponseWriter, r *http.Request) {
	u := auth.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, PortalResponse{Error: "Unauthorized"})
		return
	}

	// Get full user to get Stripe customer ID
	fullUser, err := h.userService.GetByID(r.Context(), u.UserID)
	if err != nil {
		log.Printf("[BILLING_ERROR] Failed to get user: %v", err)
		writeJSON(w, http.StatusInternalServerError, PortalResponse{Error: "Failed to get user"})
		return
	}

	if fullUser.StripeCustomerID == "" {
		writeJSON(w, http.StatusBadRequest, PortalResponse{Error: "No subscription found"})
		return
	}

	returnURL := h.appURL

	session, err := h.stripeClient.CreatePortalSession(fullUser.StripeCustomerID, returnURL)
	if err != nil {
		log.Printf("[BILLING_ERROR] Failed to create portal session: %v", err)
		writeJSON(w, http.StatusInternalServerError, PortalResponse{Error: "Failed to create portal session"})
		return
	}

	writeJSON(w, http.StatusOK, PortalResponse{URL: session.URL})
}

// Webhook handles Stripe webhook events
func (h *BillingHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[WEBHOOK_ERROR] Failed to read body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Verify signature
	signature := r.Header.Get("Stripe-Signature")
	if err := h.stripeClient.VerifyWebhookSignature(payload, signature); err != nil {
		log.Printf("[WEBHOOK_ERROR] Invalid signature: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	event, err := h.stripeClient.ParseWebhookEvent(payload)
	if err != nil {
		log.Printf("[WEBHOOK_ERROR] Failed to parse event: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("[WEBHOOK] Received event: %s", event.Type)

	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutCompleted(r, event)
	case "customer.subscription.updated":
		h.handleSubscriptionUpdated(r, event)
	case "customer.subscription.deleted":
		h.handleSubscriptionDeleted(r, event)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"received": true})
}

func (h *BillingHandler) handleCheckoutCompleted(r *http.Request, event *stripe.WebhookEvent) {
	session, err := h.stripeClient.GetCheckoutSessionFromEvent(event)
	if err != nil {
		log.Printf("[WEBHOOK_ERROR] Failed to parse checkout session: %v", err)
		return
	}

	userID := session.ClientReferenceID
	if userID == "" {
		log.Printf("[WEBHOOK_ERROR] No client_reference_id in checkout session")
		return
	}

	// Update user subscription
	// Subscription period end will be updated by subscription.updated event
	err = h.userService.UpdateSubscription(r.Context(), userID, session.Customer, user.TierPro, 0)
	if err != nil {
		log.Printf("[WEBHOOK_ERROR] Failed to update user subscription: %v", err)
		return
	}

	log.Printf("[WEBHOOK] User %s upgraded to Pro", userID)
}

func (h *BillingHandler) handleSubscriptionUpdated(r *http.Request, event *stripe.WebhookEvent) {
	sub, err := h.stripeClient.GetSubscriptionFromEvent(event)
	if err != nil {
		log.Printf("[WEBHOOK_ERROR] Failed to parse subscription: %v", err)
		return
	}

	// Find user by Stripe customer ID
	u, err := h.userService.GetByStripeCustomerID(r.Context(), sub.Customer)
	if err != nil {
		log.Printf("[WEBHOOK_ERROR] User not found for customer %s: %v", sub.Customer, err)
		return
	}

	// Determine tier based on subscription status
	tier := user.TierFree
	if sub.Status == "active" || sub.Status == "trialing" {
		tier = user.TierPro
	}

	err = h.userService.UpdateSubscription(r.Context(), u.UserID, sub.Customer, tier, sub.CurrentPeriodEnd)
	if err != nil {
		log.Printf("[WEBHOOK_ERROR] Failed to update subscription: %v", err)
		return
	}

	log.Printf("[WEBHOOK] Updated subscription for user %s: tier=%s, expires=%d", u.UserID, tier, sub.CurrentPeriodEnd)
}

func (h *BillingHandler) handleSubscriptionDeleted(r *http.Request, event *stripe.WebhookEvent) {
	sub, err := h.stripeClient.GetSubscriptionFromEvent(event)
	if err != nil {
		log.Printf("[WEBHOOK_ERROR] Failed to parse subscription: %v", err)
		return
	}

	// Find user by Stripe customer ID
	u, err := h.userService.GetByStripeCustomerID(r.Context(), sub.Customer)
	if err != nil {
		log.Printf("[WEBHOOK_ERROR] User not found for customer %s: %v", sub.Customer, err)
		return
	}

	// Downgrade to free tier
	err = h.userService.UpdateSubscription(r.Context(), u.UserID, sub.Customer, user.TierFree, 0)
	if err != nil {
		log.Printf("[WEBHOOK_ERROR] Failed to downgrade subscription: %v", err)
		return
	}

	log.Printf("[WEBHOOK] Subscription cancelled for user %s", u.UserID)
}
