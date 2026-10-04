package enforcement

import (
	"testing"
	"time"
)

var now = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC) // a Monday

func TestImmediatePermanentGroundsNeedNamedHuman(t *testing.T) {
	for _, c := range []Conduct{Minor, NCII, Coercion, ExposedMember, CourtOrder, FalseDeclaration} {
		if _, err := Decide(c, G5ModelConfirmed, 0, 0, "", now); err != ErrNoHuman {
			t.Fatalf("%s without a human: got %v", c, err)
		}
		d, err := Decide(c, G5ModelConfirmed, 0, 0, "staff-7", now)
		if err != nil || d.Action != Permanent || !d.RetainAnchors {
			t.Fatalf("%s: %+v %v", c, d, err)
		}
	}
}

func TestLadderForOtherConduct(t *testing.T) {
	want := []Action{Warn, ShadowLimit, Suspend, Suspend}
	for strikes, a := range want {
		d, err := Decide(Harassment, "", strikes, 1, "", now)
		if err != nil || d.Action != a {
			t.Fatalf("strikes %d: got %v %v", strikes, d.Action, err)
		}
	}
}

func TestGroundsThatSkipTheLadder(t *testing.T) {
	for _, g := range []Ground{G1SelfDeclared, G3CourtOrder, G4LawEnforcement} {
		d, err := Decide(Harassment, g, 0, 0, "staff-1", now)
		if err != nil || d.Action != Permanent {
			t.Fatalf("%s: %v %v", g, d.Action, err)
		}
	}
}

func TestSingleUncorroboratedReportCannotRemove(t *testing.T) {
	if _, err := Decide(Coercion, "", 0, 1, "staff-1", now); err != ErrUncorroborated {
		t.Fatalf("got %v", err)
	}
	if d, err := Decide(Coercion, "", 0, 2, "staff-1", now); err != nil || d.Action != Permanent {
		t.Fatalf("two independent reporters should suffice: %v %v", d.Action, err)
	}
}

func TestAppealNeedsDifferentReviewerAndUndoesEverything(t *testing.T) {
	d, _ := Decide(NCII, G5ModelConfirmed, 0, 0, "staff-1", now)
	if _, err := ResolveAppeal(d, "staff-1", true); err != ErrSameReviewer {
		t.Fatalf("got %v", err)
	}
	o, err := ResolveAppeal(d, "staff-2", true)
	if err != nil || !o.RestoreAccount || !o.RefundLosses || !o.DestroyAnchors {
		t.Fatalf("%+v %v", o, err)
	}
}

func TestAppealDueIsTenWorkingDays(t *testing.T) {
	if got := AppealDue(now); !got.Equal(time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("got %v", got)
	}
}
