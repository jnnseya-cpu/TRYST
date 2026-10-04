package payments

// BitriPay adapter (docs/spec/09_Operating_System.md §8; FR-084, D-26).
//
// BitriPay is integrated behind the same Provider interface as Stripe and the high-risk
// acquirer, so it can be switched on per market without touching entitlements. It is
// sandbox-only for TRYST today: its rails serve markets TRYST will never enter (01 §4.3,
// D-13), so live keys are refused unless a market is enabled for both TRYST and BitriPay
// in configuration with a recorded legal opinion.
//
// Wire facts used here are from the BitriPay developer documentation supplied by the
// founder: base URL https://api.bitripay.com/v1, Bearer sk_/rk_ keys (test and live share
// the base URL), Idempotency-Key on every money-moving POST, POST /payment_intents
// (amount_minor, currency, description → checkout_url, qr_payload, client_secret),
// POST /refunds, POST /subscriptions/{id}/cancel, the BitriPay-Signature webhook header.
// The exact refund field names and the signature header layout are not in that summary and
// are marked ASSUMED below; confirm them against the BitriPay API reference in P0.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BitriPayDefaultBase is the documented base URL.
const BitriPayDefaultBase = "https://api.bitripay.com/v1"

// Price is a catalogue entry the adapter can charge (amount in minor units).
type Price struct {
	AmountMinor int64
	Currency    string
	Description string // neutral; never names TRYST (ToS 16.2)
}

// BitriPay is the adapter.
type BitriPay struct {
	BaseURL       string
	Key           string // sk_test_… / rk_test_… or live equivalents
	WebhookSecret string // whsec_… returned once by POST /webhook_endpoints
	Market        string // ISO country the charge is for
	LiveMarkets   map[string]bool
	Prices        map[string]Price
	// SubscriptionFor maps TRYST's opaque billing reference to the BitriPay subscription id.
	SubscriptionFor func(billingRef string) (string, error)
	HTTP            *http.Client
}

var (
	ErrLiveNotEnabled = errors.New("bitripay: live keys are not enabled for this market")
	ErrUnknownPrice   = errors.New("bitripay: unknown price")
	ErrBadKey         = errors.New("bitripay: key must be an sk_ or rk_ key")
	ErrDescriptor     = errors.New("bitripay: description must be neutral")
)

// APIError is a non-2xx answer from BitriPay.
type APIError struct {
	Status int
	Code   string
}

func (e *APIError) Error() string { return fmt.Sprintf("bitripay: http %d %s", e.Status, e.Code) }

func (b *BitriPay) Name() string { return "bitripay" }

func (b *BitriPay) live() bool {
	return strings.HasPrefix(b.Key, "sk_live_") || strings.HasPrefix(b.Key, "rk_live_")
}

// check refuses publishable keys server-side and live keys outside enabled markets.
func (b *BitriPay) check() error {
	if !(strings.HasPrefix(b.Key, "sk_") || strings.HasPrefix(b.Key, "rk_")) {
		return ErrBadKey
	}
	if b.live() && !b.LiveMarkets[b.Market] {
		return ErrLiveNotEnabled
	}
	return nil
}

func (b *BitriPay) base() string {
	if b.BaseURL == "" {
		return BitriPayDefaultBase
	}
	return strings.TrimRight(b.BaseURL, "/")
}

// idempotencyKey is deterministic per logical operation, so a retry can never charge twice.
func idempotencyKey(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "tryst-" + hex.EncodeToString(h[:16])
}

func (b *BitriPay) post(ctx context.Context, path, idem string, body any, out any) error {
	if err := b.check(); err != nil {
		return err
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.base()+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+b.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idem)
	c := b.HTTP
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		var e struct {
			Code  string `json:"code"`
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		code := e.Code
		if code == "" {
			code = e.Error.Code
		}
		return &APIError{Status: resp.StatusCode, Code: code}
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

// CreateCheckout creates a payment intent and returns its hosted checkout URL. Only the
// opaque billing reference travels as the idempotency seed; nothing about the member does.
func (b *BitriPay) CreateCheckout(ctx context.Context, billingRef, priceID string) (string, error) {
	p, ok := b.Prices[priceID]
	if !ok {
		return "", ErrUnknownPrice
	}
	if !ValidDescriptor(p.Description) {
		return "", ErrDescriptor
	}
	var out struct {
		ID          string `json:"id"`
		CheckoutURL string `json:"checkout_url"`
	}
	err := b.post(ctx, "/payment_intents", idempotencyKey("checkout", billingRef, priceID),
		map[string]any{"amount_minor": p.AmountMinor, "currency": p.Currency, "description": p.Description}, &out)
	if err != nil {
		return "", err
	}
	if out.CheckoutURL == "" {
		return "", errors.New("bitripay: no checkout_url in response")
	}
	return out.CheckoutURL, nil
}

// CancelSubscription cancels at period end (one-step cancel, ToS cl. 16).
func (b *BitriPay) CancelSubscription(ctx context.Context, billingRef string) error {
	if b.SubscriptionFor == nil {
		return errors.New("bitripay: no subscription mapping")
	}
	id, err := b.SubscriptionFor(billingRef)
	if err != nil {
		return err
	}
	return b.post(ctx, "/subscriptions/"+id+"/cancel", idempotencyKey("cancel", id), map[string]any{}, nil)
}

// Refund refunds part or all of a captured intent. ASSUMED field names: payment_intent,
// amount_minor.
func (b *BitriPay) Refund(ctx context.Context, paymentID string, amountMinor int64) error {
	if amountMinor <= 0 {
		return errors.New("bitripay: refund amount must be positive")
	}
	return b.post(ctx, "/refunds", idempotencyKey("refund", paymentID, fmt.Sprint(amountMinor)),
		map[string]any{"payment_intent": paymentID, "amount_minor": amountMinor}, nil)
}

// BitriPayTolerance is the replay window for webhook timestamps.
const BitriPayTolerance = 5 * time.Minute

// VerifyWebhook checks the BitriPay-Signature header: HMAC-SHA256 of the endpoint secret.
// ASSUMED layout "t=<unix>,v1=<hex>" over "<t>.<payload>" (the Stripe convention BitriPay's
// docs follow elsewhere). The second check the docs describe, the platform Ed25519 key from
// GET /v1/keys, is added in P0 once the header carrying it is confirmed.
func (b *BitriPay) VerifyWebhook(payload []byte, header string, now time.Time) error {
	if b.WebhookSecret == "" {
		return ErrSignatureMismatch
	}
	return VerifyStripeSignature(payload, header, b.WebhookSecret, now)
}

var _ Provider = (*BitriPay)(nil)
