package admission

import (
	"slices"
	"testing"
)

var pep = Pepper("test-pepper")

func TestOnePersonOneAccount(t *testing.T) {
	x := NewIndex()
	alice := Derive(pep, "P1234567", []string{"phoneA"}, "card1")
	if v := x.Register("acc-alice", alice, ""); !v.Allowed {
		t.Fatal(v)
	}
	// Same passport, new phone and card: a second account is refused.
	again := Derive(pep, "p1234567 ", []string{"phoneZ"}, "card9")
	if v := x.Register("acc-alice-2", again, ""); v.Allowed {
		t.Fatal("second account on the same ID document")
	}
	// Re-verification of the same account is fine.
	if v := x.Check("acc-alice", alice, ""); !v.Allowed || len(v.Review) != 0 {
		t.Fatal(v)
	}
	// After erasure the person can start again.
	x.Remove("acc-alice", alice)
	if v := x.Register("acc-alice-new", again, ""); !v.Allowed {
		t.Fatal(v)
	}
	if v := x.Check("x", Anchors{}, ""); v.Allowed {
		t.Fatal("no document anchor must not pass")
	}
}

func TestControlNetworksAreFlaggedOrRefused(t *testing.T) {
	x := NewIndex()
	x.Register("a", Derive(pep, "D1", []string{"ph1"}, "cardX"), "b")
	// Verified partner on the same card: allowed, not flagged.
	if v := x.Register("b", Derive(pep, "D2", []string{"ph2"}, "cardX"), "a"); !v.Allowed || len(v.Review) != 0 {
		t.Fatalf("couple sharing a card: %+v", v)
	}
	// A third account on the same card: refused (one payer, many profiles).
	v := x.Register("c", Derive(pep, "D3", []string{"ph3"}, "cardX"), "")
	if v.Allowed || !slices.Contains(v.Review, "card_pays_for_too_many_accounts") {
		t.Fatalf("third account on one card: %+v", v)
	}
	// Someone else's phone: allowed but sent to review.
	v = x.Register("d", Derive(pep, "D4", []string{"ph1"}, "cardD"), "")
	if !v.Allowed || !slices.Contains(v.Review, "device_shared_with_other_account") {
		t.Fatalf("shared phone: %+v", v)
	}
	// A non-partner sharing a card: allowed (2 max) but reviewed.
	x2 := NewIndex()
	x2.Register("e", Derive(pep, "D5", nil, "cardE"), "")
	v = x2.Register("f", Derive(pep, "D6", nil, "cardE"), "")
	if !v.Allowed || !slices.Contains(v.Review, "card_shared_with_non_partner") {
		t.Fatalf("non-partner card share: %+v", v)
	}
}
