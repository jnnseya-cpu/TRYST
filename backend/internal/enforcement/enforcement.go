// Package enforcement implements the suspension/removal ladder and appeals
// (docs/spec/03_Backend.md §8; ToS cl. 17-18; FR-035).
package enforcement

import (
	"errors"
	"strings"
	"time"
)

type Action int

const (
	Warn Action = iota
	ShadowLimit
	Suspend
	Permanent
	LawEnforcementReferral
)

// Ground is a "declared unsafe" removal ground (01 §8.2, 02 §2.4 unsafe_ground).
type Ground string

const (
	G1SelfDeclared   Ground = "g1_self_declared"
	G2Corroborated   Ground = "g2_corroborated_reports"
	G3CourtOrder     Ground = "g3_court_order"
	G4LawEnforcement Ground = "g4_law_enforcement"
	G5ModelConfirmed Ground = "g5_model_confirmed"
	G6FalseAttest    Ground = "g6_false_attestation"
)

// Immediate-permanent conduct under ToS cl. 17.3 (anchors retained under cl. 15.3).
type Conduct string

const (
	Minor             Conduct = "minor"
	NCII              Conduct = "ncii"
	Coercion          Conduct = "coercion"
	ExposedMember     Conduct = "exposed_member"
	CourtOrder        Conduct = "court_order"
	FalseDeclaration  Conduct = "false_declaration" // ToS cl. 3.5
	Harassment        Conduct = "harassment"
	RetaliatoryReport Conduct = "retaliatory_report" // ToS cl. 17.5
)

var immediatePermanent = map[Conduct]bool{
	Minor: true, NCII: true, Coercion: true, ExposedMember: true, CourtOrder: true, FalseDeclaration: true,
}

// SkipsLadder reports grounds that go straight to permanent removal (01 §8.2).
func SkipsLadder(g Ground) bool {
	return g == G1SelfDeclared || g == G3CourtOrder || g == G4LawEnforcement
}

var (
	ErrNoHuman        = errors.New("permanent removal requires a named human decision-maker")
	ErrSameReviewer   = errors.New("appeal must be reviewed by a different person")
	ErrUncorroborated = errors.New("one uncorroborated report cannot cause a permanent removal")
)

type Decision struct {
	Action        Action
	Ground        Ground
	DecidedBy     string // staff id; required for Permanent (ToS 17.2)
	RetainAnchors bool
	DecidedAt     time.Time
}

// Decide maps conduct, ground and prior strikes to an action. Reports alone never escalate
// past a strike unless corroborated (two independent reporters) or otherwise grounded.
func Decide(c Conduct, g Ground, priorStrikes, independentReporters int, decidedBy string, now time.Time) (Decision, error) {
	if g == "" && independentReporters < 2 && immediatePermanent[c] {
		return Decision{}, ErrUncorroborated
	}
	d := Decision{Ground: g, DecidedBy: strings.TrimSpace(decidedBy), DecidedAt: now}
	switch {
	case immediatePermanent[c] || SkipsLadder(g):
		d.Action, d.RetainAnchors = Permanent, true
	case priorStrikes >= 2:
		d.Action = Suspend
	case priorStrikes == 1:
		d.Action = ShadowLimit
	default:
		d.Action = Warn
	}
	if d.Action == Permanent && d.DecidedBy == "" {
		return Decision{}, ErrNoHuman
	}
	return d, nil
}

// AppealSLA is 10 working days (ToS 18.1).
func AppealDue(filed time.Time) time.Time {
	t, n := filed, 0
	for n < 10 {
		t = t.AddDate(0, 0, 1)
		if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
			n++
		}
	}
	return t
}

// Outcome of an appeal.
type Outcome struct {
	Upheld         bool
	RestoreAccount bool
	RefundLosses   bool
	DestroyAnchors bool
}

// ResolveAppeal enforces a different reviewer and, if the member wins, restore + refund +
// destroy Exclusion Identifiers (ToS 18.2).
func ResolveAppeal(original Decision, reviewer string, memberWins bool) (Outcome, error) {
	r := strings.TrimSpace(reviewer)
	if r == "" || r == original.DecidedBy {
		return Outcome{}, ErrSameReviewer
	}
	if !memberWins {
		return Outcome{Upheld: true}, nil
	}
	return Outcome{RestoreAccount: true, RefundLosses: true, DestroyAnchors: original.RetainAnchors}, nil
}
