package commercesvc

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/payments"
)

// DevProvider stands in for the payment processor in the dev environment only: checkout
// goes to a local "test payment" page and completing it produces the same events the
// processor would send. New refuses it outside dev.
type DevProvider struct {
	mu       sync.Mutex
	sessions map[string][2]string // session -> billing ref, item
	// Calls records cancellations and refunds (tests and the dev log).
	Calls []string
}

func NewDevProvider() *DevProvider { return &DevProvider{sessions: map[string][2]string{}} }

func (d *DevProvider) Name() string { return "dev" }

func (d *DevProvider) CreateCheckout(_ context.Context, ref, key string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	id := randomRef("dcs_", 8)
	d.sessions[id] = [2]string{ref, key}
	return "/billing/checkout?session=" + id, nil
}

func (d *DevProvider) record(c string) {
	d.mu.Lock()
	d.Calls = append(d.Calls, c)
	d.mu.Unlock()
	log.Printf("DEV PAYMENTS: %s", c)
}

func (d *DevProvider) CancelSubscription(_ context.Context, ref string) error {
	d.record("cancel_at_period_end " + ref)
	return nil
}

func (d *DevProvider) CancelSubscriptionNow(_ context.Context, ref string) error {
	d.record("cancel_now " + ref)
	return nil
}

func (d *DevProvider) Refund(_ context.Context, pi string, amount int64) error {
	d.record("refund " + pi + " " + itoa(amount))
	return nil
}

// VerifyWebhook refuses everything: dev events come only from the dev checkout page.
func (d *DevProvider) VerifyWebhook([]byte, string, time.Time) error {
	return payments.ErrSignatureMismatch
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

var _ payments.Provider = (*DevProvider)(nil)
var _ ImmediateCanceller = (*DevProvider)(nil)

// handleDevComplete completes a dev checkout as if the processor had confirmed payment.
func (s *Service) handleDevComplete(w http.ResponseWriter, r *http.Request) {
	if s.dev == nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	if _, ok := s.member(w, r); !ok {
		return
	}
	var in struct {
		Session string `json:"session"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in)
	s.dev.mu.Lock()
	sess, ok := s.dev.sessions[in.Session]
	delete(s.dev.sessions, in.Session)
	s.dev.mu.Unlock()
	if !ok {
		fail(w, 404, "not_found", "That test payment has expired.")
		return
	}
	it, _ := item(sess[1])
	pi := randomRef("pi_dev_", 8)
	obj := map[string]any{"client_reference_id": sess[0], "metadata": meta{sess[0], it.Key}, "amount_total": it.PriceMinor,
		"payment_status": "paid", "payment_intent": pi, "mode": "payment"}
	if it.subscription {
		obj["mode"] = "subscription"
		obj["subscription"] = randomRef("sub_dev_", 8)
	}
	raw, _ := json.Marshal(obj)
	ev := event{ID: randomRef("evt_dev_", 8), Type: "checkout.session.completed"}
	ev.Data.Object = raw
	if _, err := s.process(r.Context(), ev); err != nil {
		fail(w, 500, "retry", "Please try again.")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
