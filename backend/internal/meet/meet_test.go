package meet

import (
	"slices"
	"testing"
	"time"
)

var (
	t0   = time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC)
	salt = []byte("device-salt")
)

func TestDoorWordsMatchOnBothPhonesOnly(t *testing.T) {
	secret := []byte("handshake-secret-shared-by-the-two-phones")
	a, b := DoorWords(secret, "meet-1"), DoorWords(secret, "meet-1")
	if a != b {
		t.Fatal("both phones must show the same words")
	}
	if DoorWords([]byte("guess"), "meet-1") == a {
		t.Fatal("an outsider produced the same words")
	}
	if DoorWords(secret, "meet-2") == a {
		t.Fatal("each meet gets new words")
	}
}

func plan(t *testing.T, trusted, line bool) *Plan {
	t.Helper()
	p, err := NewPlan(t0, t0.Add(3*time.Hour), time.Hour, "2468", "1357", salt, trusted, line)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPlanRules(t *testing.T) {
	if _, err := NewPlan(t0, t0, time.Hour, "1111", "1111", salt, true, false); err != ErrPinsMustDiffer {
		t.Fatal(err)
	}
	if _, err := NewPlan(t0, t0, time.Hour, "1", "2", salt, false, false); err != ErrNoEscalation {
		t.Fatal(err)
	}
	if _, err := NewPlan(t0, t0, 5*time.Minute, "1", "2", salt, true, false); err != ErrInterval {
		t.Fatal(err)
	}
}

func TestMissedCheckInEscalates(t *testing.T) {
	s := Start(plan(t, true, true), salt)
	if a := s.Tick(t0.Add(65 * time.Minute)); a != nil {
		t.Fatalf("inside grace: %v", a)
	}
	if a := s.Tick(t0.Add(71 * time.Minute)); !slices.Equal(a, []Action{AlertTrusted}) {
		t.Fatalf("missed: %v", a)
	}
	if a := s.Tick(t0.Add(80 * time.Minute)); a != nil {
		t.Fatalf("no repeat alerts: %v", a)
	}
	if a := s.Tick(t0.Add(87 * time.Minute)); !slices.Equal(a, []Action{AlertSafetyLine}) {
		t.Fatalf("safety team after 15 min: %v", a)
	}
}

func TestCheckInResetsTimer(t *testing.T) {
	s := Start(plan(t, true, false), salt)
	if s.CheckIn("2468", t0.Add(50*time.Minute)) != "Thanks, checked in." {
		t.Fatal("check-in")
	}
	if a := s.Tick(t0.Add(100 * time.Minute)); a != nil {
		t.Fatalf("timer should restart from the check-in: %v", a)
	}
	if s.CheckIn("0000", t0) == "Thanks, checked in." {
		t.Fatal("wrong PIN accepted")
	}
}

func TestDuressLooksNormalButEscalatesAtOnce(t *testing.T) {
	s := Start(plan(t, true, true), salt)
	if got := s.CheckIn("1357", t0.Add(20*time.Minute)); got != "Thanks, checked in." {
		t.Fatalf("duress must look like a normal check-in: %q", got)
	}
	a := s.Tick(t0.Add(21 * time.Minute))
	if !slices.Contains(a, AlertTrusted) || !slices.Contains(a, AlertSafetyLine) {
		t.Fatalf("duress escalates immediately to both: %v", a)
	}
	// A later normal check-in by someone who saw the PIN does not cancel it.
	s.CheckIn("2468", t0.Add(25*time.Minute))
	if s.End("1357") {
		t.Fatal("the duress PIN cannot end the meet")
	}
}

func TestPanicAndSafetyLineOnly(t *testing.T) {
	s := Start(plan(t, false, true), salt)
	s.Panic()
	if a := s.Tick(t0.Add(time.Minute)); !slices.Contains(a, Emergency) {
		t.Fatalf("panic: %v", a)
	}
	s2 := Start(plan(t, false, true), salt)
	if a := s2.Tick(t0.Add(75 * time.Minute)); !slices.Equal(a, []Action{AlertSafetyLine}) {
		t.Fatalf("no trusted contact: safety team first: %v", a)
	}
	s3 := Start(plan(t, true, false), salt)
	s3.End("2468")
	if a := s3.Tick(t0.Add(5 * time.Hour)); a != nil {
		t.Fatalf("ended meet: %v", a)
	}
}
