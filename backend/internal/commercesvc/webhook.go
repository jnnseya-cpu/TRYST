package commercesvc

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/keys"
)

// event is the part of a Stripe event TRYST reads.
type event struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type meta struct {
	BillingRef string `json:"billing_ref"`
	Item       string `json:"item"`
}

// handleWebhook accepts processor events: signature first, then idempotency on the event id.
// Entitlements change only here (and in the one-step cancel).
func (s *Service) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || s.cfg.Provider == nil {
		fail(w, 400, "bad_request", "Bad request")
		return
	}
	if err := s.cfg.Provider.VerifyWebhook(body, r.Header.Get("Stripe-Signature"), s.now()); err != nil {
		log.Printf("webhook rejected: %v", err)
		fail(w, 400, "bad_signature", "Bad signature")
		return
	}
	var ev event
	if json.Unmarshal(body, &ev) != nil || ev.ID == "" {
		fail(w, 400, "bad_request", "Bad request")
		return
	}
	dup, err := s.process(r.Context(), ev)
	if err != nil {
		log.Printf("webhook %s failed: %v", ev.Type, err)
		fail(w, 500, "retry", "Retry later")
		return
	}
	writeJSON(w, 200, map[string]any{"received": true, "duplicate": dup})
}

// process applies one verified event exactly once.
func (s *Service) process(ctx context.Context, ev event) (bool, error) {
	if _, err := s.cfg.Store.Get(ctx, "event", ev.ID); err == nil {
		return true, nil
	}
	s.mu.Lock()
	if s.seen[ev.ID] { // a concurrent delivery of the same event
		s.mu.Unlock()
		return true, nil
	}
	s.seen[ev.ID] = true
	s.apply(ev)
	s.mu.Unlock()
	s.flush(ctx)
	b, _ := json.Marshal(map[string]any{"type": ev.Type, "at": s.now()})
	return false, s.cfg.Store.Put(ctx, "event", ev.ID, b)
}

func unixTime(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(n, 0).UTC()
}

// apply updates state for one event. Caller holds s.mu.
func (s *Service) apply(ev event) {
	now := s.now()
	switch ev.Type {
	case "checkout.session.completed":
		var o struct {
			Mode          string `json:"mode"`
			PaymentStatus string `json:"payment_status"`
			ClientRef     string `json:"client_reference_id"`
			Metadata      meta   `json:"metadata"`
			PaymentIntent string `json:"payment_intent"`
			Subscription  string `json:"subscription"`
			AmountTotal   int64  `json:"amount_total"`
		}
		if json.Unmarshal(ev.Data.Object, &o) != nil {
			return
		}
		ref := o.Metadata.BillingRef
		if ref == "" {
			ref = o.ClientRef
		}
		id, a := s.byBillingRef(ref)
		it, ok := item(o.Metadata.Item)
		if a == nil || !ok {
			log.Printf("checkout for unknown reference or item ignored (%s)", ev.ID)
			return
		}
		if o.Mode == "payment" {
			if o.PaymentStatus != "paid" || o.PaymentIntent == "" {
				return // asynchronous payment methods settle later
			}
			l := keys.Restore(a.Ledger)
			l.CreditOnce(o.PaymentIntent, it.Keys, it.Key)
			a.Ledger = l.Entries()
			a.Payments[o.PaymentIntent] = paymentRec{Item: it.Key, Amount: o.AmountTotal, At: now}
			s.byPay[o.PaymentIntent] = id
			if it.Kind == "deposit" {
				a.Deposit = o.PaymentIntent
			}
			s.notify(id, "payment")
		} else if o.Subscription != "" {
			sb := s.subFor(id, a, it.Key, o.Subscription)
			if sb.Started.IsZero() {
				sb.Started = now
			}
			if sb.Status == "" || sb.Status == "incomplete" {
				sb.Status = "active"
			}
			if sb.PeriodEnd.IsZero() {
				sb.PeriodEnd = nextPeriod(now, it.Period)
			}
			if sb.Amount == 0 {
				sb.Amount = o.AmountTotal
			}
			if sb.Payment == "" && o.PaymentIntent != "" {
				sb.Payment = o.PaymentIntent
			}
			s.notify(id, "payment")
		}
		s.dirty[id] = true

	case "invoice.paid":
		var o struct {
			Subscription  string `json:"subscription"`
			PaymentIntent string `json:"payment_intent"`
			AmountPaid    int64  `json:"amount_paid"`
			Lines         struct {
				Data []struct {
					Period struct {
						End int64 `json:"end"`
					} `json:"period"`
				} `json:"data"`
			} `json:"lines"`
			SubscriptionDetails struct {
				Metadata meta `json:"metadata"`
			} `json:"subscription_details"`
		}
		if json.Unmarshal(ev.Data.Object, &o) != nil || o.Subscription == "" {
			return
		}
		id, a, sb := s.findSub(o.Subscription, o.SubscriptionDetails.Metadata)
		if sb == nil {
			return
		}
		sb.Status = "active"
		if o.PaymentIntent != "" {
			sb.Payment = o.PaymentIntent
			a.Payments[o.PaymentIntent] = paymentRec{Item: sb.Item, Amount: o.AmountPaid, At: now}
			s.byPay[o.PaymentIntent] = id
		}
		if o.AmountPaid > 0 {
			sb.Amount = o.AmountPaid
		}
		if len(o.Lines.Data) > 0 && o.Lines.Data[0].Period.End > 0 {
			sb.PeriodEnd = unixTime(o.Lines.Data[0].Period.End)
		}
		s.dirty[id] = true

	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		var o struct {
			ID                string `json:"id"`
			Status            string `json:"status"`
			CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
			CurrentPeriodEnd  int64  `json:"current_period_end"`
			Metadata          meta   `json:"metadata"`
			Items             struct {
				Data []struct {
					CurrentPeriodEnd int64 `json:"current_period_end"`
				} `json:"data"`
			} `json:"items"`
		}
		if json.Unmarshal(ev.Data.Object, &o) != nil {
			return
		}
		id, _, sb := s.findSub(o.ID, o.Metadata)
		if sb == nil {
			return
		}
		if sb.Status == "canceled" && ev.Type != "customer.subscription.deleted" {
			return // a cooling-off cancellation already ended it
		}
		end := o.CurrentPeriodEnd
		if end == 0 && len(o.Items.Data) > 0 {
			end = o.Items.Data[0].CurrentPeriodEnd
		}
		if end > 0 && sb.Status != "canceled" {
			sb.PeriodEnd = unixTime(end)
		}
		sb.CancelAtEnd = o.CancelAtPeriodEnd
		switch {
		case ev.Type == "customer.subscription.deleted", o.Status == "canceled", o.Status == "incomplete_expired", o.Status == "unpaid":
			if sb.Status != "canceled" {
				s.notify(id, "plan_ended")
			}
			sb.Status = "canceled"
			if sb.PeriodEnd.After(now) || sb.PeriodEnd.IsZero() {
				sb.PeriodEnd = now
			}
		case o.Status != "":
			sb.Status = o.Status
		}
		s.dirty[id] = true

	case "invoice.upcoming":
		// Sent by the processor ahead of renewal (configured to at least 7 days, 03 §10). The
		// processor also emails the billing address; TRYST holds no email to send from.
		var o struct {
			Subscription        string `json:"subscription"`
			SubscriptionDetails struct {
				Metadata meta `json:"metadata"`
			} `json:"subscription_details"`
		}
		if json.Unmarshal(ev.Data.Object, &o) != nil {
			return
		}
		if id, _, sb := s.findSub(o.Subscription, o.SubscriptionDetails.Metadata); sb != nil && !sb.CancelAtEnd {
			s.notify(id, "renewal")
		}

	case "charge.refunded", "charge.dispute.created":
		var o struct {
			PaymentIntent  string `json:"payment_intent"`
			Amount         int64  `json:"amount"`
			AmountRefunded int64  `json:"amount_refunded"`
		}
		if json.Unmarshal(ev.Data.Object, &o) != nil || o.PaymentIntent == "" {
			return
		}
		id, ok := s.byPay[o.PaymentIntent]
		if !ok {
			return
		}
		a := s.accounts[id]
		rec := a.Payments[o.PaymentIntent]
		it, _ := item(rec.Item)
		share := it.Keys
		if ev.Type == "charge.refunded" && o.Amount > 0 && o.AmountRefunded < o.Amount {
			share = int(int64(it.Keys) * o.AmountRefunded / o.Amount)
		}
		if ev.Type == "charge.dispute.created" {
			a.Disputes++
			log.Printf("chargeback opened on a %s payment", rec.Item)
		}
		if share > 0 {
			l := keys.Restore(a.Ledger)
			l.DebitUpTo(ev.Type+":"+o.PaymentIntent, share, "payment_reversed")
			a.Ledger = l.Entries()
		}
		if it.Kind == "deposit" && (ev.Type == "charge.dispute.created" || o.AmountRefunded >= o.Amount) {
			a.Deposit = ""
		}
		s.dirty[id] = true
	}
}

func (s *Service) notify(account, kind string) {
	if s.cfg.Notify != nil {
		s.cfg.Notify(account, kind, "/billing")
	}
}

func (s *Service) byBillingRef(ref string) (string, *account) {
	id, ok := s.byRef[ref]
	if !ok {
		return "", nil
	}
	return id, s.accounts[id]
}

// subFor returns (creating if needed) the record for a subscription id. Caller holds s.mu.
func (s *Service) subFor(id string, a *account, key, subID string) *sub {
	sb := a.Subs[key]
	if sb == nil || (sb.ID != "" && sb.ID != subID) {
		sb = &sub{Item: key}
		if p := a.Pending[key]; p != nil {
			sb.StartNow, sb.NoticeAt = p.StartNow, p.NoticeAt
		}
		a.Subs[key] = sb
	}
	delete(a.Pending, key)
	sb.ID = subID
	s.bySub[subID] = id
	return sb
}

// findSub finds a subscription by id, or by its metadata when events arrive out of order.
func (s *Service) findSub(subID string, m meta) (string, *account, *sub) {
	if id, ok := s.bySub[subID]; ok {
		a := s.accounts[id]
		for _, sb := range a.Subs {
			if sb.ID == subID {
				return id, a, sb
			}
		}
	}
	id, a := s.byBillingRef(m.BillingRef)
	if a == nil {
		return "", nil, nil
	}
	if _, ok := item(m.Item); !ok {
		return "", nil, nil
	}
	sb := s.subFor(id, a, m.Item, subID)
	if sb.Started.IsZero() {
		sb.Started = s.now()
	}
	return id, a, sb
}

func nextPeriod(t time.Time, period string) time.Time {
	if period == "year" {
		return t.AddDate(1, 0, 0)
	}
	return t.AddDate(0, 1, 0)
}
