package launch

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
)

func small() *Registry { return New(LaunchCities).WithGates(2.2, 10, 50) }

var (
	straightMan    = Profile{Is: []string{"man"}, Seeks: []string{"woman"}}
	straightWoman  = Profile{Is: []string{"woman"}, Seeks: []string{"man"}}
	gayMan         = Profile{Is: []string{"man"}, Seeks: []string{"man"}}
	coupleForThird = Profile{Is: []string{"couple"}, Seeks: []string{"single"}}
	singleWoman    = Profile{Is: []string{"woman", "single"}, Seeks: []string{"man", "couple"}}
)

func TestFourCitiesOpenForSignUp(t *testing.T) {
	r := New(LaunchCities)
	for _, c := range []string{"LON", "MAN", "BHM", "BTN"} {
		if st, err := r.Join(c, "m-"+c, straightWoman); err != nil || !st.Admitted {
			t.Fatalf("%s: %+v %v", c, st, err)
		}
	}
	if _, err := r.Join("DUB", "x", straightMan); !errors.Is(err, ErrUnknownCity) {
		t.Fatalf("Dublin opens in P3: %v", err)
	}
	if _, err := r.Join("LON", "m-LON", straightMan); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := r.Join("LON", "nobody", Profile{Is: []string{"man"}}); !errors.Is(err, ErrNoSeeks) {
		t.Fatalf("no seeks: %v", err)
	}
}

func TestSeedingAdmitsEveryone(t *testing.T) {
	r := small()
	for i := 0; i < 10; i++ {
		if st, _ := r.Join("LON", fmt.Sprint("m", i), straightMan); !st.Admitted {
			t.Fatalf("seed member %d waitlisted", i)
		}
	}
}

// The rule is about the group being sought, not the applicant: it holds back men seeking
// women when women are overwhelmed, and equally holds back women seeking men when men are.
func TestRuleIsSymmetric(t *testing.T) {
	for _, tc := range []struct {
		name          string
		scarce, heavy Profile
	}{{"women overwhelmed", straightWoman, straightMan}, {"men overwhelmed", straightMan, straightWoman}} {
		r := small()
		for i := 0; i < 3; i++ {
			r.Join("LON", fmt.Sprint("s", i), tc.scarce)
		}
		admitted := 0
		for i := 0; i < 12; i++ {
			if st, _ := r.Join("LON", fmt.Sprint("h", i), tc.heavy); st.Admitted {
				admitted++
			}
		}
		// seeding to 10 (3 + 7), then 7/3 = 2.33 > 2.2 already: nobody more
		if admitted != 7 {
			t.Fatalf("%s: admitted %d", tc.name, admitted)
		}
		st, _ := r.StatusOf("h7")
		if st.Admitted || st.WaitlistPosition != 1 || len(st.BusyGroups) != 1 {
			t.Fatalf("%s: first waiting: %+v", tc.name, st)
		}
	}
}

func TestSameRuleProtectsGayAndThirdPools(t *testing.T) {
	r := small()
	for i := 0; i < 10; i++ { // seed with 10 gay men: men pool load 1.0
		r.Join("MAN", fmt.Sprint("g", i), gayMan)
	}
	if st, _ := r.Join("MAN", "g10", gayMan); !st.Admitted {
		t.Fatal("a gay man adds supply as well as demand; the pool stays balanced")
	}
	// Couples seeking a single third, before any single has joined: an empty group takes at
	// most LoadCap seekers, so the first single to arrive never faces more than the cap.
	r.Join("MAN", "c1", coupleForThird)
	r.Join("MAN", "c2", coupleForThird)
	if st, _ := r.Join("MAN", "c3", coupleForThird); st.Admitted || len(st.BusyGroups) != 1 || st.BusyGroups[0] != "single" {
		t.Fatalf("third couple should wait for singles: %+v", st)
	}
	// Singles arrive and add supply: the waiting couple is released automatically.
	r.Join("MAN", "w1", singleWoman)
	if st, _ := r.StatusOf("c3"); st.Admitted {
		t.Fatalf("3 couples for 1 single is over the cap: %+v", st)
	}
	r.Join("MAN", "w2", singleWoman)
	if st, _ := r.StatusOf("c3"); !st.Admitted {
		t.Fatalf("couple should have been released: %+v", st)
	}
}

func TestNoQueueJumping(t *testing.T) {
	r := small()
	for i := 0; i < 3; i++ {
		r.Join("BTN", fmt.Sprint("w", i), straightWoman)
	}
	for i := 0; i < 9; i++ {
		r.Join("BTN", fmt.Sprint("m", i), straightMan)
	}
	// 3 women, 7 admitted men (over the cap after seeding). Two men leave: 5 men, room for one.
	for _, m := range []string{"m0", "m1"} {
		if err := r.Leave(m); err != nil {
			t.Fatal(err)
		}
	}
	if st, _ := r.StatusOf("m7"); !st.Admitted {
		t.Fatalf("longest-waiting man should be admitted: %+v", st)
	}
	if st, _ := r.Join("BTN", "late", straightMan); st.Admitted {
		t.Fatal("newcomer jumped the queue")
	}
}

func TestSeedingNeverReopens(t *testing.T) {
	r := small() // floor 10
	r.Join("LON", "w0", straightWoman)
	r.Join("LON", "w1", straightWoman)
	for i := 0; i < 8; i++ {
		r.Join("LON", fmt.Sprint("m", i), straightMan)
	}
	r.Join("LON", "late", straightMan)
	if st, _ := r.StatusOf("late"); st.Admitted {
		t.Fatal("should wait")
	}
	r.Leave("m0")
	r.Leave("m1") // 8 members < floor
	if st, _ := r.StatusOf("late"); st.Admitted {
		t.Fatal("departures reopened seeding")
	}
}

func TestCitiesGoLiveInOrder(t *testing.T) {
	r := small() // gate 50
	both := Profile{Is: []string{"couple"}, Seeks: []string{"couple"}}
	for i := 0; i < 50; i++ {
		r.Join("MAN", fmt.Sprint("man", i), both)
	}
	if st, _ := r.StatusOf("man0"); st.MatchingLive || st.NeededForLive != 0 {
		t.Fatalf("Manchester is full but must wait for London: %+v", st)
	}
	for i := 0; i < 50; i++ {
		r.Join("LON", fmt.Sprint("lon", i), both)
	}
	if st, _ := r.StatusOf("lon0"); !st.MatchingLive {
		t.Fatal("London should be live")
	}
	if st, _ := r.StatusOf("man0"); !st.MatchingLive {
		t.Fatal("Manchester should follow London")
	}
	r.Join("BHM", "b", both)
	if st, _ := r.StatusOf("b"); st.MatchingLive || st.NeededForLive != 49 {
		t.Fatalf("Birmingham not yet: %+v", st)
	}
	for i := 0; i < 5; i++ {
		r.Leave(fmt.Sprint("lon", i))
	}
	if st, _ := r.StatusOf("lon10"); !st.MatchingLive {
		t.Fatal("a live city must not close when members leave")
	}
	live := map[string]bool{}
	for _, c := range r.Report() {
		live[c.Code] = c.MatchingLive
	}
	if !live["LON"] || !live["MAN"] || live["BHM"] || live["BTN"] {
		t.Fatalf("report: %+v", live)
	}
}

// Property: after seeding, no admission pushes any group's load above the cap (or higher
// than it already was), whatever mix of people arrives and leaves.
func TestLoadInvariantRandomArrivals(t *testing.T) {
	profiles := []Profile{straightMan, straightMan, straightMan, straightWoman, gayMan, coupleForThird, singleWoman,
		{Is: []string{"woman"}, Seeks: []string{"woman"}}, {Is: []string{"nonbinary"}, Seeks: []string{"woman", "nonbinary"}}}
	rng := rand.New(rand.NewSource(11))
	for trial := 0; trial < 200; trial++ {
		r := New([]City{{"LON", "London"}}).WithGates(2.2, 20, 100)
		var ids []string
		for i := 0; i < 500; i++ {
			before := loads(r)
			seeded := r.Report()[0].Verified >= 20
			id := fmt.Sprint(trial, "-", i)
			ids = append(ids, id)
			if _, err := r.Join("LON", id, profiles[rng.Intn(len(profiles))]); err != nil {
				t.Fatal(err)
			}
			after := loads(r)
			if seeded {
				for g, l := range after {
					if l > 2.2+1e-9 && l > before[g]+1e-9 && before[g] != 0 {
						t.Fatalf("trial %d step %d: group %s load %.2f (was %.2f)", trial, i, g, l, before[g])
					}
				}
			}
			if rng.Float64() < 0.05 && len(ids) > 1 {
				_ = r.Leave(ids[rng.Intn(len(ids)-1)])
			}
		}
	}
}

func loads(r *Registry) map[string]float64 {
	out := map[string]float64{}
	for _, g := range r.Report()[0].Groups {
		if g.Supply > 0 {
			out[g.Group] = g.Load
		}
	}
	return out
}
