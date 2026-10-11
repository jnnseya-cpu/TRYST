package couple

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC)

func good() Evidence {
	return Evidence{
		A:                Partner{ProfileID: "a", V2: true, Adult: true, DocAnchor: []byte{1}, DeviceHash: "da"},
		B:                Partner{ProfileID: "b", V2: true, Adult: true, DocAnchor: []byte{2}, DeviceHash: "db"},
		PartnerFaceScore: 0.08,
		Capture:          JointCapture{LivenessPassed: true, MatchA: 0.96, MatchB: 0.94, FaceCount: 2, CapturedAt: t0.Add(-3 * time.Minute)},
		Pairing:          Pairing{ScannedAt: t0.Add(-2 * time.Minute), AApprovedAt: t0.Add(-110 * time.Second), BApprovedAt: t0.Add(-100 * time.Second)},
	}
}

func TestRealCoupleVerifies(t *testing.T) {
	if err := Verify(good(), t0); err != nil {
		t.Fatal(err)
	}
	if got := Label(t0); got != "Couple · both partners verified Oct 2026" {
		t.Fatal(got)
	}
}

func TestFakeCouplesAreCaught(t *testing.T) {
	cases := map[string]func(*Evidence){
		"one person, two accounts (same face)": func(e *Evidence) { e.PartnerFaceScore = 0.93 },
		"one person, two accounts (same ID)":   func(e *Evidence) { e.B.DocAnchor = []byte{1} },
		"one phone for both":                   func(e *Evidence) { e.B.DeviceHash = "da" },
		"partner never verified":               func(e *Evidence) { e.B.V2 = false },
		"partner under 18":                     func(e *Evidence) { e.B.Adult = false },
		"partner borrowed from a photo":        func(e *Evidence) { e.Capture.LivenessPassed = false },
		"partner not in the photo":             func(e *Evidence) { e.Capture.MatchB = 0.30 },
		"third person in the photo":            func(e *Evidence) { e.Capture.FaceCount = 3 },
		"old photo":                            func(e *Evidence) { e.Capture.CapturedAt = t0.Add(-2 * time.Hour) },
		"remote pairing, approved later":       func(e *Evidence) { e.Pairing.BApprovedAt = t0.Add(10 * time.Minute) },
		"already in another couple":            func(e *Evidence) { e.A.InActiveCouple = true },
		"no ID anchor":                         func(e *Evidence) { e.A.DocAnchor = nil },
	}
	for name, mutate := range cases {
		e := good()
		mutate(&e)
		err := Verify(e, t0)
		if !errors.Is(err, ErrNotVerified) {
			t.Errorf("%s: passed", name)
		}
	}
	e := good()
	e.B.DeviceHash, e.Capture.FaceCount = "da", 1
	if err := Verify(e, t0); err == nil || !strings.Contains(err.Error(), "own phone") || !strings.Contains(err.Error(), "exactly two faces") {
		t.Fatalf("every failure should be listed: %v", err)
	}
}

func TestReconfirmation(t *testing.T) {
	day := 24 * time.Hour
	if s := Status(t0.Add(-30*day), t0.Add(-10*day), false, t0); s != Active {
		t.Fatal(s)
	}
	if s := Status(t0.Add(-95*day), t0.Add(-10*day), false, t0); s != ReconfirmDue {
		t.Fatal("one stale partner is enough to ask both:", s)
	}
	if s := Status(t0.Add(-120*day), t0, false, t0); s != Paused {
		t.Fatal(s)
	}
	if s := Status(t0, t0, true, t0); s != Dissolved {
		t.Fatal(s)
	}
}
