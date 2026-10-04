package launch

import (
	"errors"
	"math"
	"testing"
)

func TestBreakEvenMatchesSpec(t *testing.T) {
	// E-03 table: £8.18 at 11%, £11.90 at 16%, £13.38 at 18% (V2 52%).
	for _, c := range []struct{ payer, want float64 }{{0.11, 818}, {0.16, 1190}, {0.18, 1338}} {
		if got := BreakEvenPence(c.payer, 0.52); math.Abs(got-c.want) > 1 {
			t.Errorf("payer %.2f: %.1fp, want %.0fp", c.payer, got, c.want)
		}
	}
	if BreakEvenPence(0, 0) != BreakEvenPence(0.11, 0.52) {
		t.Error("defaults")
	}
}

func TestBrakeStopsAfterTwoWeeksOver(t *testing.T) {
	b := NewBrake("cfo")
	if d := b.Evaluate(Week{City: "LON", PaidPence: 700_00, NewVerified: 100}); !d.PaidAllowed { // £7.00
		t.Fatalf("under break-even: %+v", d)
	}
	if d := b.Evaluate(Week{City: "LON", PaidPence: 1000_00, NewVerified: 100}); !d.PaidAllowed { // £10, first week over
		t.Fatalf("one week over is a warning: %+v", d)
	}
	if d := b.Evaluate(Week{City: "LON", PaidPence: 1000_00, NewVerified: 100}); d.PaidAllowed {
		t.Fatalf("second week over must stop: %+v", d)
	}
	// Stays stopped even if the next week looks fine, until a person restarts it.
	if d := b.Evaluate(Week{City: "LON", PaidPence: 0, NewVerified: 300}); d.PaidAllowed {
		t.Fatalf("restart is a human decision: %+v", d)
	}
	if err := b.Restart("LON", "growth-agent"); !errors.Is(err, ErrNotApprover) {
		t.Fatalf("an agent cannot restart spend: %v", err)
	}
	if err := b.Restart("LON", "cfo"); err != nil {
		t.Fatal(err)
	}
	if d := b.Evaluate(Week{City: "LON", PaidPence: 500_00, NewVerified: 100}); !d.PaidAllowed {
		t.Fatalf("after restart: %+v", d)
	}
}

func TestBrakeHardStopAndNoSignups(t *testing.T) {
	b := NewBrake("cfo")
	if d := b.Evaluate(Week{City: "MAN", PaidPence: 1300_00, NewVerified: 100}); d.PaidAllowed { // £13 > 1.5 × £8.18
		t.Fatalf("hard stop: %+v", d)
	}
	if d := b.Evaluate(Week{City: "BHM", PaidPence: 50_00, NewVerified: 0}); d.PaidAllowed || d.BlendedPence != -1 {
		t.Fatalf("spend with no sign-ups: %+v", d)
	}
	if d := b.Evaluate(Week{City: "BTN", PaidPence: 0, NewVerified: 0}); !d.PaidAllowed {
		t.Fatalf("no spend is fine: %+v", d)
	}
}

func TestBetterPayerRateRaisesTheLine(t *testing.T) {
	b := NewBrake()
	// £11 per sign-up is over the base case but under break-even at a 16% payer rate (£11.90).
	for i := 0; i < 3; i++ {
		if d := b.Evaluate(Week{City: "LON", PaidPence: 1100_00, NewVerified: 100, PayerRate: 0.16}); !d.PaidAllowed {
			t.Fatalf("week %d: %+v", i, d)
		}
	}
}

func TestCitiesAreIndependent(t *testing.T) {
	b := NewBrake()
	b.Evaluate(Week{City: "MAN", PaidPence: 2000_00, NewVerified: 10})
	if d := b.Evaluate(Week{City: "LON", PaidPence: 500_00, NewVerified: 100}); !d.PaidAllowed {
		t.Fatalf("London unaffected by Manchester: %+v", d)
	}
}
