// Package payments is the processor-agnostic payment layer (docs/spec/03_Backend.md §10;
// FR-078, D-22). Stripe is primary (subject to written approval); a high-risk acquirer is
// the live backup. Entitlements never depend on a single processor.
package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Provider is implemented by each processor adapter.
type Provider interface {
	Name() string
	CreateCheckout(ctx context.Context, billingRef string, priceID string) (url string, err error)
	CancelSubscription(ctx context.Context, billingRef string) error
	Refund(ctx context.Context, paymentID string, amountMinor int64) error
	VerifyWebhook(payload []byte, signatureHeader string, now time.Time) error
}

// Descriptor rules (ToS 16.2): neutral, truthful, never "TRYST", includes a service URL.
func ValidDescriptor(d string) bool {
	if d == "" || len(d) > 22 {
		return false
	}
	return !strings.Contains(strings.ToUpper(d), "TRYST")
}

var (
	ErrBadSignatureHeader = errors.New("malformed signature header")
	ErrSignatureMismatch  = errors.New("signature mismatch")
	ErrTimestampTolerance = errors.New("timestamp outside tolerance")
)

// StripeTolerance is the replay window for webhook timestamps.
const StripeTolerance = 5 * time.Minute

// VerifyStripeSignature checks a Stripe-Signature header ("t=<unix>,v1=<hex>[,v1=...]"):
// v1 = HMAC-SHA256(secret, "<t>.<payload>"). Implemented with the standard library so the
// check is auditable; the Stripe SDK can replace it once the account is approved.
func VerifyStripeSignature(payload []byte, header, secret string, now time.Time) error {
	var ts int64 = -1
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return ErrBadSignatureHeader
			}
			ts = n
		case "v1":
			sigs = append(sigs, v)
		}
	}
	if ts < 0 || len(sigs) == 0 {
		return ErrBadSignatureHeader
	}
	if d := now.Sub(time.Unix(ts, 0)); d > StripeTolerance || d < -StripeTolerance {
		return ErrTimestampTolerance
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte("."))
	mac.Write(payload)
	want := mac.Sum(nil)
	for _, s := range sigs {
		got, err := hex.DecodeString(s)
		if err == nil && hmac.Equal(got, want) {
			return nil
		}
	}
	return ErrSignatureMismatch
}

// Dedup makes webhook processing idempotent on event id.
type Dedup struct{ seen map[string]bool }

func NewDedup() *Dedup { return &Dedup{seen: map[string]bool{}} }

// First reports whether this event id is new.
func (d *Dedup) First(eventID string) bool {
	if d.seen[eventID] {
		return false
	}
	d.seen[eventID] = true
	return true
}
