// Package launch implements public sign-up across several launch cities with the city
// density gate and the gender-balance waitlist (docs/spec/01_Product.md §11; FR-089,
// FR-090, D-35).
//
//   - Sign-up is open to every verified adult in a launch city; no invitation is needed.
//   - Matching in a city switches on once it has DensityGate verified members (G-City-1).
//     Before that, members verify and set up their profile and see how close the city is.
//   - Balance (G-City-2): among solo members who declared themselves men or women, the
//     larger side may not exceed RatioCap times the smaller. A member whose admission would
//     breach the cap joins a first-come, first-served waitlist for that city and is
//     admitted as soon as the balance allows. Couples and members of any other gender are
//     never waitlisted. Pricing and ranking never use gender (D-06; 03 §3).
//   - While a city is seeding (fewer than SeedFloor balanced solos), nobody is waitlisted.
package launch

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Defaults from 01 §11 and §13.1.
const (
	DensityGate = 2000
	RatioCap    = 2.2
	SeedFloor   = 200
)

// Cohort is the balance group a member falls into. It is derived from the member's own
// declaration for this purpose only and is never shown to other members.
type Cohort int

const (
	Men Cohort = iota
	Women
	Other  // any other gender identity: never waitlisted
	Couple // couple profiles: never waitlisted
)

func (c Cohort) String() string { return [...]string{"men", "women", "other", "couple"}[c] }

// City is a launch city.
type City struct {
	Code, Name string
}

// LaunchCities opens together at launch (founder, v1.3). Dublin follows with the Irish
// launch in P3.
var LaunchCities = []City{
	{"LON", "London"}, {"MAN", "Manchester"}, {"BHM", "Birmingham"}, {"BTN", "Brighton"},
}

// Status is what a member is told about their place.
type Status struct {
	Admitted         bool
	WaitlistPosition int // 1-based; 0 when admitted
	MatchingLive     bool
	VerifiedInCity   int
	NeededForLive    int // members still needed before matching opens; 0 when live
}

type cityState struct {
	seeded  bool // set once the seed floor is reached; never reset, so departures cannot reopen seeding
	counts  [4]int
	waiting []waiter
	where   map[string]int // member → index in waiting (rebuilt on release)
}

type membership struct {
	city   string
	cohort Cohort
}

type waiter struct {
	member string
	cohort Cohort
}

// Registry tracks admissions per city. Safe for concurrent use.
type Registry struct {
	mu     sync.Mutex
	cities map[string]*cityState
	member map[string]membership
	cap    float64
	floor  int
	gate   int
}

var (
	ErrUnknownCity = errors.New("not a launch city")
	ErrDuplicate   = errors.New("member already registered")
	ErrNotFound    = errors.New("member not found")
)

// New returns a registry for the given cities with the default gates.
func New(cities []City) *Registry {
	r := &Registry{cities: map[string]*cityState{}, member: map[string]membership{},
		cap: RatioCap, floor: SeedFloor, gate: DensityGate}
	for _, c := range cities {
		r.cities[c.Code] = &cityState{where: map[string]int{}}
	}
	return r
}

// WithGates overrides the gates (tests, per-market configuration).
func (r *Registry) WithGates(ratioCap float64, seedFloor, densityGate int) *Registry {
	r.cap, r.floor, r.gate = ratioCap, seedFloor, densityGate
	return r
}

func (s *cityState) verified() int {
	return s.counts[Men] + s.counts[Women] + s.counts[Other] + s.counts[Couple]
}

// fits reports whether admitting one more of cohort keeps the city within the cap.
func (r *Registry) fits(s *cityState, c Cohort) bool {
	if c == Other || c == Couple {
		return true
	}
	m, w := s.counts[Men], s.counts[Women]
	if !s.seeded && m+w < r.floor {
		return true
	}
	if c == Men {
		m++
	} else {
		w++
	}
	big, small := m, w
	if c == Women {
		big, small = w, m
	}
	if big <= small { // admitting the smaller (or equal) side never worsens balance
		return true
	}
	return float64(big) <= r.cap*float64(small)
}

// Join registers a newly verified member. It returns their status.
func (r *Registry) Join(city, member string, c Cohort) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.cities[city]
	if !ok {
		return Status{}, ErrUnknownCity
	}
	if _, dup := r.member[member]; dup {
		return Status{}, ErrDuplicate
	}
	r.member[member] = membership{city, c}
	if !cohortWaiting(s, c) && r.fits(s, c) {
		s.counts[c]++
		r.releaseLocked(s) // a new arrival on the smaller side may free waiting members
		return r.statusLocked(s, member), nil
	}
	// First come, first served: never jump members already waiting in this cohort.
	s.waiting = append(s.waiting, waiter{member, c})
	r.reindex(s)
	r.releaseLocked(s)
	return r.statusLocked(s, member), nil
}

// releaseLocked admits waiting members in arrival order while the balance allows.
// A waiter who still does not fit does not block members of the other cohort behind them.
func (r *Registry) releaseLocked(s *cityState) {
	defer r.markSeeded(s)
	r.markSeeded(s)
	for {
		admitted := false
		blocked := map[Cohort]bool{}
		for i := 0; i < len(s.waiting); i++ {
			w := s.waiting[i]
			if blocked[w.cohort] {
				continue
			}
			if r.fits(s, w.cohort) {
				s.counts[w.cohort]++
				r.markSeeded(s)
				s.waiting = append(s.waiting[:i], s.waiting[i+1:]...)
				admitted = true
				break
			}
			blocked[w.cohort] = true
		}
		if !admitted {
			break
		}
	}
	r.reindex(s)
}

func (r *Registry) markSeeded(s *cityState) {
	if s.counts[Men]+s.counts[Women] >= r.floor {
		s.seeded = true
	}
}

// cohortWaiting reports whether anyone of this cohort is already waiting (first come,
// first served within a cohort).
func cohortWaiting(s *cityState, c Cohort) bool {
	for _, w := range s.waiting {
		if w.cohort == c {
			return true
		}
	}
	return false
}

func (r *Registry) reindex(s *cityState) {
	s.where = make(map[string]int, len(s.waiting))
	for i, w := range s.waiting {
		s.where[w.member] = i
	}
}

func (r *Registry) statusLocked(s *cityState, member string) Status {
	v := s.verified()
	st := Status{Admitted: true, MatchingLive: v >= r.gate, VerifiedInCity: v}
	if !st.MatchingLive {
		st.NeededForLive = r.gate - v
	}
	if i, waiting := s.where[member]; waiting {
		st.Admitted = false
		// position counts only members of the same cohort ahead of them
		pos := 1
		for _, w := range s.waiting[:i] {
			if w.cohort == s.waiting[i].cohort {
				pos++
			}
		}
		st.WaitlistPosition = pos
	}
	return st
}

// StatusOf returns a member's current status.
func (r *Registry) StatusOf(member string) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.member[member]
	if !ok {
		return Status{}, ErrNotFound
	}
	return r.statusLocked(r.cities[m.city], member), nil
}

// Leave removes an admitted or waiting member (erasure, account closure) and releases
// waiting members if the balance now allows.
func (r *Registry) Leave(member string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.member[member]
	if !ok {
		return ErrNotFound
	}
	s, c := r.cities[m.city], m.cohort
	delete(r.member, member)
	if i, waiting := s.where[member]; waiting {
		s.waiting = append(s.waiting[:i], s.waiting[i+1:]...)
		r.reindex(s)
		return nil
	}
	if s.counts[c] == 0 {
		return fmt.Errorf("launch: cohort %s count already zero", c)
	}
	s.counts[c]--
	r.releaseLocked(s)
	return nil
}

// CityReport is the operator view (aggregates only, never member-level).
type CityReport struct {
	Code         string
	Verified     int
	Men, Women   int
	Waiting      int
	Ratio        float64 // larger:smaller among men and women; 0 if either is zero
	MatchingLive bool
}

// Report lists every city, sorted by code.
func (r *Registry) Report() []CityReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]CityReport, 0, len(r.cities))
	for code, s := range r.cities {
		m, w := s.counts[Men], s.counts[Women]
		ratio := 0.0
		if m > 0 && w > 0 {
			big, small := m, w
			if w > m {
				big, small = w, m
			}
			ratio = float64(big) / float64(small)
		}
		out = append(out, CityReport{Code: code, Verified: s.verified(), Men: m, Women: w,
			Waiting: len(s.waiting), Ratio: ratio, MatchingLive: s.verified() >= r.gate})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}
