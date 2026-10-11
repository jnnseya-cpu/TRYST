package payments

// Stripe adapter (docs/spec/03_Backend.md §10; D-22). Plain HTTPS against the documented
// REST API (form-encoded requests, Bearer secret key, Idempotency-Key on every POST), so the
// calls are auditable without the SDK. Only the opaque billing reference and the product key
// travel to Stripe: no profile id, segment or anything about the member (03 §10).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// StripeDefaultBase is Stripe's API base URL.
const StripeDefaultBase = "https://api.stripe.com"

// StripePrice maps a TRYST catalogue key to a Stripe Price.
type StripePrice struct {
	PriceID      string // price_…
	Subscription bool   // recurring (mode=subscription) or one-off (mode=payment)
}

// Stripe is the adapter.
type Stripe struct {
	BaseURL       string
	SecretKey     string // sk_test_… / sk_live_… or a restricted rk_ key
	WebhookSecret string // whsec_…
	Prices        map[string]StripePrice
	SuccessURL    string // e.g. https://app.example/billing?done=1
	CancelURL     string
	// SubscriptionFor maps the billing reference to the Stripe subscription id.
	SubscriptionFor func(billingRef string) (string, error)
	HTTP            *http.Client
	now             func() time.Time
}

// StripeError is a non-2xx answer from Stripe.
type StripeError struct {
	Status int
	Code   string
	Type   string
}

func (e *StripeError) Error() string {
	return fmt.Sprintf("stripe: http %d %s %s", e.Status, e.Type, e.Code)
}

var ErrStripeKey = errors.New("stripe: a secret (sk_) or restricted (rk_) key is required")

func (s *Stripe) Name() string { return "stripe" }

func (s *Stripe) base() string {
	if s.BaseURL == "" {
		return StripeDefaultBase
	}
	return strings.TrimRight(s.BaseURL, "/")
}

func (s *Stripe) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Stripe) call(ctx context.Context, method, path, idem string, form url.Values, out any) error {
	if !(strings.HasPrefix(s.SecretKey, "sk_") || strings.HasPrefix(s.SecretKey, "rk_")) {
		return ErrStripeKey
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, s.base()+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.SecretKey)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	c := s.HTTP
	if c == nil {
		c = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error struct {
				Code string `json:"code"`
				Type string `json:"type"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		return &StripeError{Status: resp.StatusCode, Code: e.Error.Code, Type: e.Error.Type}
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

// CreateCheckout creates a Checkout Session and returns its URL. The idempotency key is
// stable for ten minutes, so a double click returns the same session instead of two.
func (s *Stripe) CreateCheckout(ctx context.Context, billingRef, priceKey string) (string, error) {
	p, ok := s.Prices[priceKey]
	if !ok || p.PriceID == "" {
		return "", ErrUnknownPrice
	}
	f := url.Values{}
	f.Set("line_items[0][price]", p.PriceID)
	f.Set("line_items[0][quantity]", "1")
	f.Set("client_reference_id", billingRef)
	f.Set("metadata[billing_ref]", billingRef)
	f.Set("metadata[item]", priceKey)
	f.Set("success_url", s.SuccessURL)
	f.Set("cancel_url", s.CancelURL)
	if p.Subscription {
		f.Set("mode", "subscription")
		f.Set("subscription_data[metadata][billing_ref]", billingRef)
		f.Set("subscription_data[metadata][item]", priceKey)
	} else {
		f.Set("mode", "payment")
		f.Set("payment_intent_data[metadata][billing_ref]", billingRef)
		f.Set("payment_intent_data[metadata][item]", priceKey)
	}
	bucket := s.clock().Unix() / 600
	var out struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := s.call(ctx, http.MethodPost, "/v1/checkout/sessions", idempotencyKey("checkout", billingRef, priceKey, fmt.Sprint(bucket)), f, &out); err != nil {
		return "", err
	}
	if out.URL == "" {
		return "", errors.New("stripe: no url in checkout session")
	}
	return out.URL, nil
}

func (s *Stripe) subscription(billingRef string) (string, error) {
	if s.SubscriptionFor == nil {
		return "", errors.New("stripe: no subscription mapping")
	}
	return s.SubscriptionFor(billingRef)
}

// CancelSubscription cancels at the end of the paid period (ToS 16.3): one call, no
// retention flow.
func (s *Stripe) CancelSubscription(ctx context.Context, billingRef string) error {
	id, err := s.subscription(billingRef)
	if err != nil {
		return err
	}
	f := url.Values{}
	f.Set("cancel_at_period_end", "true")
	return s.call(ctx, http.MethodPost, "/v1/subscriptions/"+url.PathEscape(id), idempotencyKey("cancel", id), f, nil)
}

// CancelSubscriptionNow ends the subscription immediately (cooling-off cancellation,
// ToS 16.4).
func (s *Stripe) CancelSubscriptionNow(ctx context.Context, billingRef string) error {
	id, err := s.subscription(billingRef)
	if err != nil {
		return err
	}
	return s.call(ctx, http.MethodDelete, "/v1/subscriptions/"+url.PathEscape(id), "", nil, nil)
}

// Refund refunds part or all of a payment to the original method (ToS 16.7).
func (s *Stripe) Refund(ctx context.Context, paymentID string, amountMinor int64) error {
	if amountMinor <= 0 {
		return errors.New("stripe: refund amount must be positive")
	}
	f := url.Values{}
	f.Set("payment_intent", paymentID)
	f.Set("amount", fmt.Sprint(amountMinor))
	return s.call(ctx, http.MethodPost, "/v1/refunds", idempotencyKey("refund", paymentID, fmt.Sprint(amountMinor)), f, nil)
}

// VerifyWebhook checks the Stripe-Signature header.
func (s *Stripe) VerifyWebhook(payload []byte, header string, now time.Time) error {
	if s.WebhookSecret == "" {
		return ErrSignatureMismatch
	}
	return VerifyStripeSignature(payload, header, s.WebhookSecret, now)
}

var _ Provider = (*Stripe)(nil)
