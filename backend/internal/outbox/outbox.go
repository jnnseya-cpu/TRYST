// Package outbox delivers messages between services durably (D-59). A message is written
// to the sender's own database in the same step as the change that caused it, then sent;
// it is deleted only once the receiver acknowledged it. A restart, a deploy or the other
// service being down delays delivery but never loses it. Receivers are idempotent.
//
// Messages with the same key replace each other (a member's latest standing is all that
// matters); messages added with Add get a unique key and are all delivered, oldest first.
package outbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/metrics"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

// Message is one queued delivery.
type Message struct {
	Key     string          `json:"key"`
	Payload json.RawMessage `json:"payload"`
	Queued  time.Time       `json:"queued"`
	Seq     int64           `json:"seq"`
}

// Box is one named outbox in a store.
type Box struct {
	Name string // e.g. "signals"
	St   store.Store
	Send func(ctx context.Context, m Message) error
	Now  func() time.Time

	mu      sync.Mutex // one delivery pass at a time
	seq     int64
	seqOnce sync.Once
}

func (b *Box) kind() string { return "outbox:" + b.Name }

func (b *Box) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

func (b *Box) nextSeq() int64 {
	b.seqOnce.Do(func() { b.seq = b.now().UnixNano() })
	b.seq++
	return b.seq
}

// Put queues payload under key, replacing an undelivered message with the same key.
func (b *Box) Put(ctx context.Context, key string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	m := Message{Key: key, Payload: raw, Queued: b.now(), Seq: b.nextSeq()}
	doc, _ := json.Marshal(m)
	return b.St.Put(ctx, b.kind(), key, doc)
}

// Add queues payload under a fresh key.
func (b *Box) Add(ctx context.Context, payload any) error {
	k := make([]byte, 12)
	_, _ = rand.Read(k)
	return b.Put(ctx, hex.EncodeToString(k), payload)
}

// Deliver sends every queued message, oldest first. A message is removed only after Send
// succeeds and only if it wasn't replaced meanwhile. Returns how many remain.
func (b *Box) Deliver(ctx context.Context) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	docs, err := b.St.All(ctx, b.kind())
	if err != nil {
		log.Printf("outbox %s: %v", b.Name, err)
		return -1
	}
	var msgs []Message
	for _, d := range docs {
		var m Message
		if json.Unmarshal(d, &m) == nil {
			msgs = append(msgs, m)
		}
	}
	sort.Slice(msgs, func(i, j int) bool { return msgs[i].Seq < msgs[j].Seq })
	left := 0
	for _, m := range msgs {
		if err := b.Send(ctx, m); err != nil {
			left++
			metrics.Default.Inc("tryst_outbox_failures_total", "box", b.Name)
			continue
		}
		cur, err := b.St.Get(ctx, b.kind(), m.Key)
		var now Message
		if err == nil && json.Unmarshal(cur, &now) == nil && now.Seq == m.Seq {
			_ = b.St.Delete(ctx, b.kind(), m.Key)
		} else if err == nil {
			left++ // replaced while we sent it; the newer one goes next pass
		}
	}
	return left
}

// Pending counts queued messages (metrics, readiness).
func (b *Box) Pending(ctx context.Context) int {
	docs, err := b.St.All(ctx, b.kind())
	if err != nil {
		return -1
	}
	return len(docs)
}

// Run delivers every interval until ctx ends.
func (b *Box) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		b.Deliver(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Decode is a helper for Send functions.
func (m Message) Decode(v any) error {
	if err := json.Unmarshal(m.Payload, v); err != nil {
		return fmt.Errorf("outbox: bad payload: %w", err)
	}
	return nil
}
