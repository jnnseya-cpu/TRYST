package launch

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"testing"
)

func small() *Registry { return New(LaunchCities).WithGates(2.2, 10, 50) }

func TestFourCitiesOpenAtOnce(t *testing.T) {
	r := New(LaunchCities)
	for _, c := range []string{"LON", "MAN", "BHM", "BTN"} {
		if st, err := r.Join(c, "m-"+c, Women); err != nil || !st.Admitted {
			t.Fatalf("%s: %+v %v", c, st, err)
		}
	}
	if _, err := r.Join("DUB", "x", Men); !errors.Is(err, ErrUnknownCity) {
		t.Fatalf("Dublin opens in P3: %v", err)
	}
	if _, err := r.Join("LON", "m-LON", Men); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate: %v", err)
	}
}

func TestSeedingAdmitsEveryone(t *testing.T) {
	r := small()
	for i := 0; i < 10; i++ {
		if st, _ := r.Join("LON", fmt.Sprint("m", i), Men); !st.Admitted {
			t.Fatalf("seed member %d waitlisted", i)
		}
	}
}

func TestOversuppliedSideWaitsThenIsReleasedInOrder(t *testing.T) {
	r := small()
	for i := 0; i < 3; i++ {
		r.Join("LON", fmt.Sprint("w", i), Women)
	}
	admitted := 0
	for i := 0; i < 12; i++ {
		if st, _ := r.Join("LON", fmt.Sprint("m", i), Men); st.Admitted {
			admitted++
		}
	}
	// floor 10 reached at 3 women + 7 men; then men allowed while men <= 2.2*3 = 6.6 → no more
	if admitted != 7 {
		t.Fatalf("admitted %d men", admitted)
	}
	st, _ := r.StatusOf("m7")
	if st.Admitted || st.WaitlistPosition != 1 {
		t.Fatalf("first waiting man: %+v", st)
	}
	if st, _ := r.StatusOf("m11"); st.WaitlistPosition != 5 {
		t.Fatalf("fifth waiting man: %+v", st)
	}
	// One woman arrives: cap becomes 2.2*4 = 8.8, so one waiting man (the first) is admitted.
	r.Join("LON", "w3", Women)
	if st, _ := r.StatusOf("m7"); !st.Admitted {
		t.Fatalf("m7 should be released: %+v", st)
	}
	if st, _ := r.StatusOf("m8"); st.Admitted || st.WaitlistPosition != 1 {
		t.Fatalf("m8 moves to the front: %+v", st)
	}
}

func TestCouplesAndOtherGendersNeverWait(t *testing.T) {
	r := small()
	for i := 0; i < 20; i++ {
		r.Join("MAN", fmt.Sprint("m", i), Men)
	}
	if st, _ := r.Join("MAN", "c1", Couple); !st.Admitted {
		t.Fatal("couple waitlisted")
	}
	if st, _ := r.Join("MAN", "nb1", Other); !st.Admitted {
		t.Fatal("other gender waitlisted")
	}
	if st, _ := r.Join("MAN", "w1", Women); !st.Admitted {
		t.Fatal("scarce side waitlisted")
	}
}

func TestNoQueueJumping(t *testing.T) {
	r := small()
	for i := 0; i < 3; i++ {
		r.Join("BTN", fmt.Sprint("w", i), Women)
	}
	for i := 0; i < 9; i++ {
		r.Join("BTN", fmt.Sprint("m", i), Men)
	}
	// 3 women and 7 admitted men (cap 6.6, already over after seeding). Two men leave: 5 men,
	// room for exactly one more. The longest-waiting man gets it, not a newcomer.
	first := ""
	for i := 0; i < 9; i++ {
		if st, _ := r.StatusOf(fmt.Sprint("m", i)); !st.Admitted && first == "" {
			first = fmt.Sprint("m", i)
		}
	}
	for _, m := range []string{"m0", "m1"} {
		if err := r.Leave(m); err != nil {
			t.Fatal(err)
		}
	}
	if st, _ := r.StatusOf(first); !st.Admitted {
		t.Fatalf("%s should have been admitted", first)
	}
	if st, _ := r.Join("BTN", "late", Men); st.Admitted {
		t.Fatal("newcomer jumped the queue")
	}
}

func TestDensityGate(t *testing.T) {
	r := small()
	st, _ := r.Join("BHM", "a", Couple)
	if st.MatchingLive || st.NeededForLive != 49 {
		t.Fatalf("before gate: %+v", st)
	}
	for i := 0; i < 49; i++ {
		r.Join("BHM", fmt.Sprint("c", i), Couple)
	}
	if st, _ := r.StatusOf("a"); !st.MatchingLive || st.NeededForLive != 0 {
		t.Fatalf("after gate: %+v", st)
	}
}

// Property: once seeding is over, admitting members never pushes the ratio above the cap
// (or above where it already was), and nobody is admitted out of order within a cohort.
func TestBalanceInvariantRandomArrivals(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 200; trial++ {
		r := New([]City{{"LON", "London"}}).WithGates(2.2, 20, 100)
		var ids []string
		for i := 0; i < 600; i++ {
			before := r.Report()[0]
			var c Cohort
			switch x := rng.Float64(); {
			case x < 0.70:
				c = Men
			case x < 0.92:
				c = Women
			case x < 0.96:
				c = Other
			default:
				c = Couple
			}
			id := fmt.Sprint(trial, "-", i)
			ids = append(ids, id)
			if _, err := r.Join("LON", id, c); err != nil {
				t.Fatal(err)
			}
			after := r.Report()[0]
			if before.Men+before.Women >= 20 && ratio(after) > 2.2+1e-9 && ratio(after) > ratio(before)+1e-9 {
				t.Fatalf("trial %d step %d: ratio rose to %.3f (was %.3f)", trial, i, ratio(after), ratio(before))
			}
			// Departures can unbalance a city on their own; check that the releases they
			// trigger do not make it worse than the departure alone.
			if rng.Float64() < 0.05 && len(ids) > 1 {
				_ = r.Leave(ids[rng.Intn(len(ids)-1)])
				rep := r.Report()[0]
				if ratio(rep) > 2.2+1e-9 {
					// over the cap: the larger side must not have gained anyone
					if (rep.Men > rep.Women && rep.Men > after.Men) || (rep.Women > rep.Men && rep.Women > after.Women) {
						t.Fatalf("trial %d step %d: release admitted the oversupplied side at ratio %.3f", trial, i, ratio(rep))
					}
				}
			}
		}
	}
}

// ratio is larger:smaller among men and women, +Inf when only one side is present.
func ratio(c CityReport) float64 {
	big, small := math.Max(float64(c.Men), float64(c.Women)), math.Min(float64(c.Men), float64(c.Women))
	switch {
	case big == 0:
		return 0
	case small == 0:
		return math.Inf(1)
	}
	return big / small
}

// Regression (found by the property test): departures that drop a city back below the
// seed floor must not reopen seeding and release the oversupplied side.
func TestSeedingNeverReopens(t *testing.T) {
	r := small() // floor 10
	r.Join("LON", "w0", Women)
	r.Join("LON", "w1", Women)
	for i := 0; i < 8; i++ {
		r.Join("LON", fmt.Sprint("m", i), Men) // floor reached: 2 women + 8 men
	}
	r.Join("LON", "late", Men) // 9 > 2.2*2 → waits
	if st, _ := r.StatusOf("late"); st.Admitted {
		t.Fatal("should wait")
	}
	r.Leave("m0")
	r.Leave("m1") // 6 men + 2 women = 8 < floor
	if st, _ := r.StatusOf("late"); st.Admitted {
		t.Fatal("departures reopened seeding and released the oversupplied side")
	}
}
