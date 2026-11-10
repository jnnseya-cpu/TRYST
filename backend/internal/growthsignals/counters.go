// Package growthsignals keeps the member platform's weekly activity counts for growth
// intelligence (D-73): (week, metric, city) -> number of distinct members.
//
// Each member counts at most once per week, metric and city, so the cohort floor is about
// people, not events. To know who was already counted this week it keeps a keyed token per
// member (HMAC under a random key made for the week); when the week ends the key and the
// tokens are discarded and only the count remains. No member id, time of day or anything
// else is kept, so a finished week links back to no one and holds nothing to erase.
//
// Export releases only complete weeks and only counts at or above the cohort floor
// (k ≥ 50 people, 03 §4.2); a smaller count is withheld, never sent as zero.
package growthsignals

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

const (
	kind      = "growth_counts"
	docID     = "weekly"
	MinCohort = 50
	keepWeeks = 12
)

// Metrics the platform counts.
const (
	TonightStart = "tonight_start"
	IntentSent   = "intent_sent"
)

// Counters is safe for concurrent use.
type Counters struct {
	mu  sync.Mutex
	st  store.Store
	now func() time.Time
	n   map[string]int // "2026-11-02|tonight_start|London" -> distinct members
	cur current
}

// current is this week's dedup state; it is replaced, never archived, when the week ends.
type current struct {
	Week string          `json:"week"`
	Key  string          `json:"key"`  // random per week
	Seen map[string]bool `json:"seen"` // HMAC(key, member|metric|city)
}

type saved struct {
	Counts  map[string]int `json:"counts"`
	Current current        `json:"current"`
}

// New loads saved counts.
func New(ctx context.Context, st store.Store, now func() time.Time) *Counters {
	if now == nil {
		now = time.Now
	}
	c := &Counters{st: st, now: now, n: map[string]int{}}
	if b, err := st.Get(ctx, kind, docID); err == nil {
		var sv saved
		if err := json.Unmarshal(b, &sv); err != nil {
			log.Printf("growth counts: %v", err)
		} else if sv.Counts != nil {
			c.n, c.cur = sv.Counts, sv.Current
		}
	}
	return c
}

// weekStart is the Monday 00:00 UTC that starts t's week.
func weekStart(t time.Time) time.Time {
	t = t.UTC()
	d := (int(t.Weekday()) + 6) % 7
	return time.Date(t.Year(), t.Month(), t.Day()-d, 0, 0, 0, 0, time.UTC)
}

// Inc counts a member in a city once per week and metric. A nil receiver does nothing.
func (c *Counters) Inc(metric, city, member string) {
	if c == nil || city == "" || member == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	week := weekStart(now).Format("2006-01-02")
	if c.cur.Week != week { // a new week: the old key and tokens are dropped
		k := make([]byte, 32)
		_, _ = rand.Read(k)
		c.cur = current{Week: week, Key: hex.EncodeToString(k), Seen: map[string]bool{}}
	}
	key, _ := hex.DecodeString(c.cur.Key)
	m := hmac.New(sha256.New, key)
	m.Write([]byte(member + "|" + metric + "|" + city))
	tok := hex.EncodeToString(m.Sum(nil))[:32]
	if c.cur.Seen[tok] {
		return
	}
	c.cur.Seen[tok] = true
	c.n[week+"|"+metric+"|"+city]++
	cutoff := weekStart(now).AddDate(0, 0, -7*keepWeeks).Format("2006-01-02")
	for k := range c.n {
		if k[:10] < cutoff {
			delete(c.n, k)
		}
	}
	b, _ := json.Marshal(saved{Counts: c.n, Current: c.cur})
	if err := c.st.Put(context.Background(), kind, docID, b); err != nil {
		log.Printf("growth counts: %v", err)
	}
}

// Item is one exported count (the growth service's observation shape).
type Item struct {
	SourceID       string    `json:"source_id"`
	SourceRecordID string    `json:"source_record_id"`
	Term           string    `json:"term"`
	Market         string    `json:"market"`
	Locale         string    `json:"locale"`
	Metric         string    `json:"metric"`
	Value          float64   `json:"value"`
	Unit           string    `json:"unit"`
	Coverage       float64   `json:"coverage"`
	WindowStart    time.Time `json:"window_start"`
	WindowEnd      time.Time `json:"window_end"`
	ObservedAt     time.Time `json:"observed_at"`
}

var sourceFor = map[string]string{TonightStart: "platform_tonight", IntentSent: "platform_intents"}

// Export returns complete weeks with counts ≥ MinCohort.
func (c *Counters) Export() []Item {
	c.mu.Lock()
	defer c.mu.Unlock()
	thisWeek := weekStart(c.now())
	out := []Item{}
	for k, n := range c.n {
		parts := strings.SplitN(k, "|", 3)
		ws, err := time.Parse("2006-01-02", parts[0])
		if err != nil || !ws.Before(thisWeek) || n < MinCohort {
			continue
		}
		src, ok := sourceFor[parts[1]]
		if !ok {
			continue
		}
		we := ws.AddDate(0, 0, 7)
		city := parts[2]
		out = append(out, Item{SourceID: src, SourceRecordID: fmt.Sprintf("%s-%s-%s", parts[1], strings.ToLower(strings.ReplaceAll(city, " ", "_")), ws.Format("20060102")),
			Term: strings.ReplaceAll(parts[1], "_", " ") + " " + city, Market: "GB", Locale: "en-GB", Metric: parts[1],
			Value: float64(n), Unit: "count", Coverage: 1, WindowStart: ws, WindowEnd: we, ObservedAt: we})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SourceRecordID < out[j].SourceRecordID })
	return out
}
