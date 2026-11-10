package growthsignals

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

// TestOnlyCompleteWeeksAboveTheCohortFloorLeave: a week with fewer than 50 events in a city
// is withheld, the current week is never sent, and nothing about any member is kept.
func TestOnlyCompleteWeeksAboveTheCohortFloorLeave(t *testing.T) {
	st := store.NewMemory()
	now := time.Date(2026, 11, 4, 15, 0, 0, 0, time.UTC) // Wednesday
	c := New(context.Background(), st, func() time.Time { return now })
	for i := range 60 {
		c.Inc(TonightStart, "London", fmt.Sprintf("acc-%d", i))
	}
	for range 200 { // one person, many times: still one
		c.Inc(TonightStart, "Brighton", "acc-same")
	}
	for i := range 48 {
		c.Inc(TonightStart, "Brighton", fmt.Sprintf("acc-b%d", i))
	}
	c.Inc(IntentSent, "", "acc-1") // no city: not counted
	if got := c.Export(); len(got) != 0 {
		t.Fatalf("the current week left: %v", got)
	}
	now = now.AddDate(0, 0, 7) // the week is now complete
	got := c.Export()
	if len(got) != 1 || got[0].Term != "tonight start London" || got[0].Value != 60 || got[0].SourceID != "platform_tonight" ||
		!got[0].WindowStart.Equal(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("export: %+v", got)
	}
	if got := c.n["2026-11-02|tonight_start|Brighton"]; got != 49 {
		t.Fatalf("Brighton counted %d people, want 49 (withheld)", got)
	}
	// A new week drops last week's key and tokens; only counts remain.
	c.Inc(IntentSent, "Manchester", "acc-1")
	raw, _ := st.Get(context.Background(), kind, docID)
	if strings.Contains(string(raw), "acc-") {
		t.Fatal("a member id was stored")
	}
	var sv saved
	_ = json.Unmarshal(raw, &sv)
	if sv.Current.Week != "2026-11-09" || len(sv.Current.Seen) != 1 {
		t.Fatalf("last week's tokens were kept: %+v", sv.Current)
	}
	for k := range sv.Counts {
		if strings.Count(k, "|") != 2 {
			t.Fatalf("stored more than week|metric|city: %q", k)
		}
	}
	// Restart: counts survive; old weeks are pruned.
	c2 := New(context.Background(), st, func() time.Time { return now.AddDate(0, 0, 7*(keepWeeks+2)) })
	c2.Inc(IntentSent, "Manchester", "acc-2")
	if len(c2.Export()) != 0 {
		t.Fatal("weeks older than the retention were kept")
	}
	var nilCounters *Counters
	nilCounters.Inc(TonightStart, "London", "acc-1") // a platform without counters does nothing
}
