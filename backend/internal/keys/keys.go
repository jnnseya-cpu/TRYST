// Package keys implements the Keys ledger and intent costing (01 §10.2, 03 §5.4, §10;
// FR-013): 5 Keys solo->solo, 15 couple->solo, fully refunded on reply; inbound cap queues
// overflow silently.
package keys

import (
	"errors"
	"time"
)

var ErrInsufficient = errors.New("keys_insufficient")

const (
	CostSoloToSolo   = 5
	CostCoupleToSolo = 15 // 3x: broadcast couple outreach is priced, not moderated
	FreeIntentsWeek  = 3
	DefaultCap       = 12
	MinCap, MaxCap   = 4, 40
)

// Cost of one outbound intent.
func Cost(fromCouple, toSolo bool) int {
	if fromCouple && toSolo {
		return CostCoupleToSolo
	}
	return CostSoloToSolo
}

// Entry is one ledger line (commerce.keys_ledger).
type Entry struct {
	Delta  int    `json:"delta"`
	Reason string `json:"reason"`
	Intent string `json:"intent,omitempty"`
	Ref    string `json:"ref,omitempty"` // payment, voucher or refund reference (idempotency)
}

// Ledger is an append-only, double-entry-style ledger for one member (in-memory model of
// commerce.keys_ledger). The balance can never go negative.
type Ledger struct {
	entries []Entry
	held    map[string]int // intent id -> keys held pending reply
}

func NewLedger() *Ledger { return &Ledger{held: map[string]int{}} }

// Restore rebuilds a ledger from its persisted entries.
func Restore(entries []Entry) *Ledger {
	l := NewLedger()
	for _, e := range entries {
		l.entries = append(l.entries, e)
		switch e.Reason {
		case "intent":
			l.held[e.Intent] = -e.Delta
		case "refund_on_reply":
			l.held[e.Intent] = 0
		}
	}
	return l
}

// Entries returns a copy of the ledger lines for persistence.
func (l *Ledger) Entries() []Entry { return append([]Entry(nil), l.entries...) }

func (l *Ledger) Balance() int {
	b := 0
	for _, e := range l.entries {
		b += e.Delta
	}
	return b
}

func (l *Ledger) Credit(n int, reason string) {
	if n > 0 {
		l.entries = append(l.entries, Entry{Delta: n, Reason: reason})
	}
}

// CreditOnce credits n Keys for a payment or voucher reference; a repeated ref is ignored.
func (l *Ledger) CreditOnce(ref string, n int, reason string) bool {
	if n <= 0 || ref == "" {
		return false
	}
	for _, e := range l.entries {
		if e.Ref == ref {
			return false
		}
	}
	l.entries = append(l.entries, Entry{Delta: n, Reason: reason, Ref: ref})
	return true
}

// DebitUpTo removes up to n Keys for a refunded or disputed payment without ever making the
// balance negative (Keys already spent stay spent). Idempotent per ref; returns the amount.
func (l *Ledger) DebitUpTo(ref string, n int, reason string) int {
	for _, e := range l.entries {
		if e.Ref == ref {
			return 0
		}
	}
	if b := l.Balance(); n > b {
		n = b
	}
	if n <= 0 {
		return 0
	}
	l.entries = append(l.entries, Entry{Delta: -n, Reason: reason, Ref: ref})
	return n
}

// Spend holds keys for an intent. Idempotent per intent id.
func (l *Ledger) Spend(intentID string, n int) error {
	if _, dup := l.held[intentID]; dup {
		return nil
	}
	if l.Balance() < n {
		return ErrInsufficient
	}
	l.entries = append(l.entries, Entry{Delta: -n, Reason: "intent", Intent: intentID})
	l.held[intentID] = n
	return nil
}

// RefundOnReply returns the full spend when the recipient replies. Idempotent.
func (l *Ledger) RefundOnReply(intentID string) {
	n, ok := l.held[intentID]
	if !ok || n == 0 {
		return
	}
	l.entries = append(l.entries, Entry{Delta: n, Reason: "refund_on_reply", Intent: intentID})
	l.held[intentID] = 0
}

// Inbox applies the recipient's inbound cap per rolling 24 h. Overflow is queued and
// re-scored later; the sender is never told (02 §5.5).
type Inbox struct {
	Cap       int
	delivered []time.Time
	Queued    []string
}

func ClampCap(c int) int {
	if c < MinCap {
		return MinCap
	}
	if c > MaxCap {
		return MaxCap
	}
	return c
}

// Offer returns true if the intent is delivered now, false if queued.
func (in *Inbox) Offer(intentID string, now time.Time) bool {
	cutoff := now.Add(-24 * time.Hour)
	kept := in.delivered[:0]
	for _, t := range in.delivered {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	in.delivered = kept
	if len(in.delivered) >= ClampCap(in.Cap) {
		in.Queued = append(in.Queued, intentID)
		return false
	}
	in.delivered = append(in.delivered, now)
	return true
}
