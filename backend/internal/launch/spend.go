package launch

// Paid-acquisition brake (01 §10.3, §11; FR-091, D-35).
//
// Public sign-up removes the invite loop that held acquisition cost down, so paid seeding is
// allowed only while it pays for itself. Each week, per city, blended cost per verified
// sign-up (all paid spend ÷ all new verified members, every channel) is compared with the
// break-even line from E-03: contribution per payer × payer rate × V2 rate (£8.18 per
// sign-up at the base case). Paid spend in the city stops automatically when cost is above
// break-even for two weeks running, or 50% above it in any single week. Stopping is
// automatic; restarting is a money decision, so it needs a named person. Referrals and
// organic search continue whatever the brake does.

import (
	"errors"
	"fmt"
	"sync"
)

// Base-case economics (01 §10.3). Replace with beta cohorts as they arrive.
const (
	ContributionPerPayerPence = 14300
	DefaultPayerRate          = 0.11
	DefaultV2Rate             = 0.52
	HardStopMultiple          = 1.5
	WeeksOverBeforeStop       = 2
)

// Week is one city's acquisition figures for one week.
type Week struct {
	City        string
	PaidPence   int64   // all paid seeding spend in the city this week
	NewVerified int     // new verified members this week, every channel
	PayerRate   float64 // observed verified→payer rate; 0 uses the base case
	V2Rate      float64 // observed sign-up→V2 rate; 0 uses the base case
}

// BreakEvenPence is the highest blended cost per verified sign-up that still breaks even.
func BreakEvenPence(payerRate, v2Rate float64) float64 {
	if payerRate <= 0 {
		payerRate = DefaultPayerRate
	}
	if v2Rate <= 0 {
		v2Rate = DefaultV2Rate
	}
	return ContributionPerPayerPence * payerRate * v2Rate
}

// SpendDecision is the brake's answer for the coming week.
type SpendDecision struct {
	PaidAllowed    bool
	BlendedPence   float64 // -1 when there was spend but no sign-ups
	BreakEvenPence float64
	Reason         string
}

// Brake holds per-city state. Safe for concurrent use.
type Brake struct {
	mu        sync.Mutex
	overWeeks map[string]int
	stopped   map[string]bool
	approvers map[string]bool
}

var ErrNotApprover = errors.New("restarting paid spend needs a named approver")

// NewBrake returns a brake; approvers are the people allowed to restart paid spend.
func NewBrake(approvers ...string) *Brake {
	b := &Brake{overWeeks: map[string]int{}, stopped: map[string]bool{}, approvers: map[string]bool{}}
	for _, a := range approvers {
		b.approvers[a] = true
	}
	return b
}

// Evaluate records a week and decides whether paid spend may continue in that city.
func (b *Brake) Evaluate(w Week) SpendDecision {
	b.mu.Lock()
	defer b.mu.Unlock()
	be := BreakEvenPence(w.PayerRate, w.V2Rate)
	d := SpendDecision{BreakEvenPence: be}
	switch {
	case w.PaidPence == 0:
		d.BlendedPence = 0
	case w.NewVerified == 0:
		d.BlendedPence = -1
	default:
		d.BlendedPence = float64(w.PaidPence) / float64(w.NewVerified)
	}
	over := d.BlendedPence < 0 || d.BlendedPence > be
	hard := d.BlendedPence < 0 || d.BlendedPence > HardStopMultiple*be
	if over {
		b.overWeeks[w.City]++
	} else {
		b.overWeeks[w.City] = 0
	}
	switch {
	case b.stopped[w.City]:
		d.Reason = "paid spend stopped; restart needs a named approver"
	case hard:
		b.stopped[w.City] = true
		d.Reason = fmt.Sprintf("stopped: cost per sign-up more than %.0f%% of break-even", HardStopMultiple*100)
	case b.overWeeks[w.City] >= WeeksOverBeforeStop:
		b.stopped[w.City] = true
		d.Reason = fmt.Sprintf("stopped: above break-even for %d weeks", WeeksOverBeforeStop)
	case over:
		d.Reason = "warning: above break-even this week"
	default:
		d.Reason = "within break-even"
	}
	d.PaidAllowed = !b.stopped[w.City]
	return d
}

// Restart lets paid spend resume in a city after a named person has reviewed it.
func (b *Brake) Restart(city, approver string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.approvers[approver] {
		return ErrNotApprover
	}
	b.stopped[city] = false
	b.overWeeks[city] = 0
	return nil
}
