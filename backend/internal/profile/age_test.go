package profile

import (
	"testing"
	"time"
)

func ym(t *testing.T, s string) BirthYM {
	t.Helper()
	b, err := ParseBirthYM(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAgeNeverOverstated(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		born string
		want int
	}{
		{"1990-03", 36}, // birthday passed
		{"1990-10", 35}, // birth month: shown one lower until the month ends
		{"1990-11", 35}, // birthday not reached
		{"2008-09", 18},
		{"2008-10", 17},
	}
	for _, c := range cases {
		if got := AgeYears(ym(t, c.born), now); got != c.want {
			t.Errorf("%s: %d, want %d", c.born, got, c.want)
		}
	}
	// Property: for every possible day of birth in the month, the shown age is <= true age.
	for y := 1950; y <= 2008; y++ {
		for m := time.January; m <= time.December; m++ {
			for d := 1; d <= 28; d++ {
				dob := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
				trueAge := now.Year() - y
				if now.YearDay() < time.Date(now.Year(), m, d, 0, 0, 0, 0, time.UTC).YearDay() {
					trueAge--
				}
				if shown := AgeYears(BirthYM{y, m}, now); shown > trueAge {
					t.Fatalf("born %s: shown %d > true %d", dob.Format("2006-01-02"), shown, trueAge)
				}
			}
		}
	}
}

func TestNoUnder18EverPasses(t *testing.T) {
	now := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	if Adult(true, ym(t, "2008-10"), now) {
		t.Fatal("18th birthday this month: wait until next month")
	}
	if Adult(true, ym(t, "2009-01"), now) {
		t.Fatal("17-year-old passed")
	}
	if !Adult(true, ym(t, "2008-09"), now) {
		t.Fatal("18-year-old refused")
	}
	if Adult(false, ym(t, "1980-01"), now) {
		t.Fatal("provider refusal must win")
	}
	if Adult(true, ym(t, "2030-01"), now) {
		t.Fatal("future birth date")
	}
	if _, err := ParseBirthYM("2008-13"); err == nil {
		t.Fatal("bad month accepted")
	}
}
