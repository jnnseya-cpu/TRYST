// Package launch implements public sign-up across the launch cities, city-by-city matching
// and the demand-balance waitlist (docs/spec/01_Product.md §11; FR-089, FR-090, D-35, D-36).
//
//   - Sign-up is open to every verified adult in a launch city; no invitation is needed.
//   - Matching opens one city at a time, in launch order (London first). A city goes live
//     once it has DensityGate verified members (G-City-1) and the city before it is live;
//     once live it stays live. Before that, members see how close their city is.
//   - Balance (G-City-2) is gender-neutral (D-36). Nobody is admitted or held because of who
//     they are. Every member is a "receiver" in the groups that describe them and a
//     "seeker" of the groups they asked to meet. A new member waits only if one of the groups
//     they want to meet is already saturated: more than LoadCap seekers per receiver. The rule
//     is identical for everyone and protects whichever group is being overwhelmed, whether that
//     is women in a straight pool, men in a gay pool or singles sought by couples.
//   - Waiting is first come, first served among people seeking the same groups, and a new
//     member who joins any group adds supply, which releases waiting members automatically.
//   - While a city is seeding (fewer than SeedFloor members), nobody waits; departures never
//     reopen seeding. Price and ranking never use any of this (D-06; 03 §3).
package launch

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

// Defaults from 01 §11 and §13.1.
const (
	DensityGate = 2000
	LoadCap     = 2.2 // seekers per receiver in any group; 2.0 from P3
	SeedFloor   = 200
)

// Profile is what the pacing rule sees: the groups a member belongs to and the groups they
// want to meet, as plain labels from their own profile ("woman", "man", "single", "couple"
// and so on). The rule never interprets a label; it only counts.
type Profile struct {
	Is    []string
	Seeks []string
}

func (p Profile) key() string {
	s := append([]string(nil), p.Seeks...)
	sort.Strings(s)
	return strings.Join(s, "|")
}

// City is a launch city.
type City struct {
	Code, Name string
}

// LaunchCities are open for sign-up from launch (founder, v1.3); matching opens in this
// order. Dublin follows with the Irish launch in P3.
var LaunchCities = []City{
	{"LON", "London"}, {"MAN", "Manchester"}, {"BHM", "Birmingham"}, {"BTN", "Brighton"},
}

// Status is what a member is told about their place.
type Status struct {
	Admitted         bool
	WaitlistPosition int      // 1-based among people seeking the same groups; 0 when admitted
	BusyGroups       []string // the groups that are full right now, so the wait is explained
	MatchingLive     bool
	VerifiedInCity   int
	NeededForLive    int // members still needed before matching can open; 0 once reached
}

type cityState struct {
	seeded   bool           // set once the seed floor is reached; never reset
	admitted int            // admitted members
	supply   map[string]int // group → admitted members who are in it
	demand   map[string]int // group → admitted members who seek it
	waiting  []waiter
}

type waiter struct {
	member string
	p      Profile
}

type membership struct {
	city string
	p    Profile
}

// Registry tracks admissions per city. Safe for concurrent use.
type Registry struct {
	mu     sync.Mutex
	cities map[string]*cityState
	order  []string        // launch order
	live   map[string]bool // sticky: once matching opens in a city it stays open
	member map[string]membership
	cap    float64
	floor  int
	gate   int
}

var (
	ErrUnknownCity = errors.New("not a launch city")
	ErrDuplicate   = errors.New("member already registered")
	ErrNotFound    = errors.New("member not found")
	ErrNoSeeks     = errors.New("a member must say who they want to meet")
)

// New returns a registry for the given cities with the default gates.
func New(cities []City) *Registry {
	r := &Registry{cities: map[string]*cityState{}, live: map[string]bool{},
		member: map[string]membership{}, cap: LoadCap, floor: SeedFloor, gate: DensityGate}
	for _, c := range cities {
		r.cities[c.Code] = &cityState{supply: map[string]int{}, demand: map[string]int{}}
		r.order = append(r.order, c.Code)
	}
	return r
}

// WithGates overrides the gates (tests, per-market configuration).
func (r *Registry) WithGates(loadCap float64, seedFloor, densityGate int) *Registry {
	r.cap, r.floor, r.gate = loadCap, seedFloor, densityGate
	return r
}

func has(list []string, g string) bool {
	for _, x := range list {
		if x == g {
			return true
		}
	}
	return false
}

// busy returns the sought groups that admitting p would push over the load cap. A group's
// load counts p's own supply if p belongs to it.
func (r *Registry) busy(s *cityState, p Profile) []string {
	if !s.seeded && s.admitted < r.floor {
		return nil
	}
	var out []string
	for _, g := range p.Seeks {
		supply := s.supply[g]
		if has(p.Is, g) {
			supply++
		}

		demand := s.demand[g] + 1
		before := 0.0
		if s.supply[g] > 0 {
			before = float64(s.demand[g]) / float64(s.supply[g])
		}
		// An empty group is treated as having one member, so at most LoadCap seekers are
		// admitted before anyone joins it: the first to join never faces more than the cap.
		after := float64(demand) / float64(max(supply, 1))
		// Over the cap, and not an improvement on where the group already was.
		if after > r.cap && !(s.supply[g] > 0 && after <= before) {
			out = append(out, g)
		}
	}
	sort.Strings(out)
	return out
}

func (r *Registry) admit(s *cityState, p Profile) {
	s.admitted++
	for _, g := range p.Is {
		s.supply[g]++
	}
	for _, g := range p.Seeks {
		s.demand[g]++
	}
	if s.admitted >= r.floor {
		s.seeded = true
	}
}

func keyWaiting(s *cityState, k string) bool {
	for _, w := range s.waiting {
		if w.p.key() == k {
			return true
		}
	}
	return false
}

// Join registers a newly verified member and returns their status.
func (r *Registry) Join(city, member string, p Profile) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.cities[city]
	if !ok {
		return Status{}, ErrUnknownCity
	}
	if len(p.Seeks) == 0 {
		return Status{}, ErrNoSeeks
	}
	if _, dup := r.member[member]; dup {
		return Status{}, ErrDuplicate
	}
	r.member[member] = membership{city, p}
	// First come, first served: never jump people already waiting for the same groups.
	if !keyWaiting(s, p.key()) && len(r.busy(s, p)) == 0 {
		r.admit(s, p)
	} else {
		s.waiting = append(s.waiting, waiter{member, p})
	}
	r.releaseLocked(s)
	return r.statusLocked(member), nil
}

// releaseLocked admits waiting members in arrival order while their groups have room.
// A waiter who still does not fit does not block people seeking other groups.
func (r *Registry) releaseLocked(s *cityState) {
	for {
		admitted := false
		blocked := map[string]bool{}
		for i := 0; i < len(s.waiting); i++ {
			w := s.waiting[i]
			k := w.p.key()
			if blocked[k] {
				continue
			}
			if len(r.busy(s, w.p)) == 0 {
				r.admit(s, w.p)
				s.waiting = append(s.waiting[:i], s.waiting[i+1:]...)
				admitted = true
				break
			}
			blocked[k] = true
		}
		if !admitted {
			return
		}
	}
}

// refreshLiveLocked opens cities in launch order: each needs the density gate and a live
// predecessor. Live is never revoked.
func (r *Registry) refreshLiveLocked() {
	for i, code := range r.order {
		if r.live[code] {
			continue
		}
		if r.cities[code].admitted >= r.gate && (i == 0 || r.live[r.order[i-1]]) {
			r.live[code] = true
			continue
		}
		return // later cities wait for this one
	}
}

func (r *Registry) statusLocked(member string) Status {
	r.refreshLiveLocked()
	m := r.member[member]
	s := r.cities[m.city]
	st := Status{Admitted: true, MatchingLive: r.live[m.city], VerifiedInCity: s.admitted}
	if s.admitted < r.gate {
		st.NeededForLive = r.gate - s.admitted
	}
	for i, w := range s.waiting {
		if w.member != member {
			continue
		}
		st.Admitted = false
		pos := 1
		for _, ahead := range s.waiting[:i] {
			if ahead.p.key() == w.p.key() {
				pos++
			}
		}
		st.WaitlistPosition = pos
		st.BusyGroups = r.busy(s, w.p)
		break
	}
	return st
}

// StatusOf returns a member's current status.
func (r *Registry) StatusOf(member string) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.member[member]; !ok {
		return Status{}, ErrNotFound
	}
	return r.statusLocked(member), nil
}

// Leave removes an admitted or waiting member (erasure, account closure) and releases
// waiting members if their groups now have room.
func (r *Registry) Leave(member string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.member[member]
	if !ok {
		return ErrNotFound
	}
	s := r.cities[m.city]
	delete(r.member, member)
	for i, w := range s.waiting {
		if w.member == member {
			s.waiting = append(s.waiting[:i], s.waiting[i+1:]...)
			return nil
		}
	}
	s.admitted--
	for _, g := range m.p.Is {
		s.supply[g]--
	}
	for _, g := range m.p.Seeks {
		s.demand[g]--
	}
	r.releaseLocked(s)
	return nil
}

// GroupLoad is one group's balance in a city.
type GroupLoad struct {
	Group          string
	Supply, Demand int
	Load           float64 // demand per member of the group; 0 when the group is empty
}

// CityReport is the operator view (aggregates only, never member-level).
type CityReport struct {
	Code         string
	Verified     int
	Waiting      int
	MatchingLive bool
	Groups       []GroupLoad
}

// Report lists every city in launch order.
func (r *Registry) Report() []CityReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refreshLiveLocked()
	out := make([]CityReport, 0, len(r.order))
	for _, code := range r.order {
		s := r.cities[code]
		groups := map[string]bool{}
		for g := range s.supply {
			groups[g] = true
		}
		for g := range s.demand {
			groups[g] = true
		}
		rep := CityReport{Code: code, Verified: s.admitted, Waiting: len(s.waiting), MatchingLive: r.live[code]}
		for g := range groups {
			gl := GroupLoad{Group: g, Supply: s.supply[g], Demand: s.demand[g]}
			if gl.Supply > 0 {
				gl.Load = float64(gl.Demand) / float64(gl.Supply)
			}
			rep.Groups = append(rep.Groups, gl)
		}
		sort.Slice(rep.Groups, func(i, j int) bool { return rep.Groups[i].Group < rep.Groups[j].Group })
		out = append(out, rep)
	}
	return out
}
