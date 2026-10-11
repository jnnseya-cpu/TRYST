package outbox

import (
	"context"
	"errors"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

func TestDurableOrderedCoalescedDelivery(t *testing.T) {
	st := store.NewMemory()
	var got []string
	down := true
	send := func(_ context.Context, m Message) error {
		if down {
			return errors.New("receiver down")
		}
		var v string
		_ = m.Decode(&v)
		got = append(got, v)
		return nil
	}
	b := &Box{Name: "t", St: st, Send: send}
	ctx := context.Background()
	b.Add(ctx, "signal-1")
	b.Put(ctx, "acc-1", "warn")
	b.Add(ctx, "signal-2")
	b.Put(ctx, "acc-1", "suspend") // replaces "warn"
	if left := b.Deliver(ctx); left != 3 {
		t.Fatalf("left while down = %d", left)
	}
	// A restart: a new Box on the same store still has everything.
	b2 := &Box{Name: "t", St: st, Send: send}
	down = false
	if left := b2.Deliver(ctx); left != 0 {
		t.Fatalf("left = %d", left)
	}
	want := []string{"signal-1", "signal-2", "suspend"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("delivered %v, want %v", got, want)
	}
	if b2.Pending(ctx) != 0 {
		t.Fatal("delivered messages kept")
	}
}

func TestReplacedWhileSendingIsNotLost(t *testing.T) {
	st := store.NewMemory()
	var b *Box
	var got []string
	first := true
	b = &Box{Name: "t", St: st, Send: func(ctx context.Context, m Message) error {
		var v string
		_ = m.Decode(&v)
		got = append(got, v)
		if first {
			first = false
			_ = b.Put(ctx, "acc-1", "good") // an appeal overturns it while we deliver
		}
		return nil
	}}
	ctx := context.Background()
	b.Put(ctx, "acc-1", "suspend")
	if left := b.Deliver(ctx); left != 1 {
		t.Fatalf("newer message dropped: left=%d", left)
	}
	b.Deliver(ctx)
	if len(got) != 2 || got[1] != "good" {
		t.Fatalf("delivered %v", got)
	}
}
