package commercesvc

import (
	"context"
	"log"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/dsr"
	"github.com/jnnseya-cpu/tryst/backend/internal/keys"
)

// Name implements dsr.Part.
func (s *Service) Name() string { return "commerce" }

// Export returns plans, payments (amounts and dates), the Keys ledger and voucher use.
func (s *Service) Export(_ context.Context, account string) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.accounts[account]
	if a == nil {
		return nil, nil
	}
	type pay struct {
		Item   string    `json:"item"`
		Amount int64     `json:"amount_minor"`
		At     time.Time `json:"at"`
	}
	var pays []pay
	for _, p := range a.Payments {
		pays = append(pays, pay{p.Item, p.Amount, p.At})
	}
	return map[string]any{
		"billing_reference": a.BillingRef, "subscriptions": a.Subs, "payments": pays, "keys_ledger": a.Ledger,
		"keys": keys.Restore(a.Ledger).Balance(), "deposit_paid": a.Deposit != "", "voucher_plan": a.Voucher,
		"note": "Card details are held only by our payment processor, never by us.",
	}, nil
}

// Erase stops all billing and deletes the commerce record. Live subscriptions end now (with
// the cooling-off refund if inside 14 days); the refundable deposit is refunded if its Keys
// are still unspent. The processor keeps its own payment records for statutory accounting.
func (s *Service) Erase(ctx context.Context, account string) (dsr.Receipt, error) {
	rc := dsr.Receipt{Part: "commerce"}
	s.mu.Lock()
	a := s.accounts[account]
	if a == nil {
		s.mu.Unlock()
		if err := s.cfg.Store.Delete(ctx, "account", account); err != nil {
			return rc, err
		}
		return rc, nil
	}
	now := s.now()
	type stop struct {
		ref, payment string
		refund       int64
	}
	var stops []stop
	for k, sb := range a.Subs {
		if sb.live(now) && !sb.CancelAtEnd {
			r := int64(0)
			if now.Sub(sb.Started) < CoolingOff {
				r = proportionateRefund(sb, now)
			}
			stops = append(stops, stop{a.BillingRef + "|" + k, sb.Payment, r})
		}
	}
	depositRefund := ""
	if a.Deposit != "" && keys.Restore(a.Ledger).Balance() >= 60 {
		depositRefund = a.Deposit
	}
	s.mu.Unlock()

	// Processor calls without the lock (the adapter resolves subscriptions through us).
	// Re-applying an erasure after a restore skips them: they happened the first time.
	if s.SkipProcessor {
		stops, depositRefund = nil, ""
	}
	if s.cfg.Provider != nil {
		for _, st := range stops {
			var err error
			if ic, ok := s.cfg.Provider.(ImmediateCanceller); ok {
				err = ic.CancelSubscriptionNow(ctx, st.ref)
			} else {
				err = s.cfg.Provider.CancelSubscription(ctx, st.ref)
			}
			if err == nil && st.refund > 0 && st.payment != "" {
				err = s.cfg.Provider.Refund(ctx, st.payment, st.refund)
			}
			if err != nil {
				return rc, err // retried: erasure is idempotent
			}
			rc.Erased++
		}
		if depositRefund != "" {
			if it, ok := item("deposit"); ok {
				if err := s.cfg.Provider.Refund(ctx, depositRefund, it.PriceMinor); err != nil {
					log.Printf("deposit refund on erasure failed: %v", err)
					return rc, err
				}
			}
		}
	} else if len(stops) > 0 {
		return rc, errProviderNeeded
	}

	s.mu.Lock()
	delete(s.byRef, a.BillingRef)
	for _, sb := range a.Subs {
		delete(s.bySub, sb.ID)
	}
	for pi := range a.Payments {
		delete(s.byPay, pi)
	}
	delete(s.accounts, account)
	delete(s.dirty, account)
	s.mu.Unlock()
	if err := s.cfg.Store.Delete(ctx, "account", account); err != nil {
		return rc, err
	}
	rc.Erased++
	if len(a.Payments) > 0 {
		rc.Retained = append(rc.Retained, "Our payment processor keeps its own record of your payments for statutory accounting; it holds no profile data")
	}
	return rc, nil
}

var _ dsr.Part = (*Service)(nil)
