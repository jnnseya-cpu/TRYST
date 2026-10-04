package keys

import (
	"fmt"
	"testing"
	"time"
)

func TestCosts(t *testing.T) {
	if Cost(true, true) != 3*Cost(false, true) {
		t.Fatal("couple->solo must cost 3x solo->solo")
	}
}

func TestRefundOnReplyIsFullAndIdempotent(t *testing.T) {
	l := NewLedger()
	l.Credit(60, "pack")
	if err := l.Spend("i1", CostCoupleToSolo); err != nil {
		t.Fatal(err)
	}
	if err := l.Spend("i1", CostCoupleToSolo); err != nil || l.Balance() != 45 {
		t.Fatalf("spend must be idempotent, balance %d", l.Balance())
	}
	l.RefundOnReply("i1")
	l.RefundOnReply("i1")
	if l.Balance() != 60 {
		t.Fatalf("balance %d, want 60", l.Balance())
	}
}

func TestBalanceNeverNegative(t *testing.T) {
	l := NewLedger()
	l.Credit(9, "deposit")
	for i := 0; i < 5; i++ {
		_ = l.Spend(fmt.Sprintf("i%d", i), CostSoloToSolo)
		if l.Balance() < 0 {
			t.Fatal("negative balance")
		}
	}
	if err := l.Spend("x", CostSoloToSolo); err != ErrInsufficient {
		t.Fatalf("got %v", err)
	}
}

func TestInboundCapQueuesOverflow(t *testing.T) {
	in := &Inbox{Cap: DefaultCap}
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	delivered := 0
	for i := 0; i < 20; i++ {
		if in.Offer(fmt.Sprintf("i%d", i), now) {
			delivered++
		}
	}
	if delivered != 12 || len(in.Queued) != 8 {
		t.Fatalf("delivered %d queued %d", delivered, len(in.Queued))
	}
	if !in.Offer("later", now.Add(25*time.Hour)) {
		t.Fatal("window should roll after 24h")
	}
}

func TestCapBounds(t *testing.T) {
	if ClampCap(1) != 4 || ClampCap(99) != 40 {
		t.Fatal("cap must stay within 4-40")
	}
}
