package growth

import (
	"math"
	"sort"
)

// Scoring v1 (spec §5, §17): full precision, no rounding before a decision.
const (
	ScoringVersion  = "v1.0"
	singleFamilyCap = 69.0
	minHistory      = 4
)

func clamp(x float64) float64 { return math.Max(0, math.Min(100, x)) }

func median(xs []float64) float64 {
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func mad(xs []float64) float64 {
	m := median(xs)
	d := make([]float64, len(xs))
	for i, x := range xs {
		d[i] = math.Abs(x - m)
	}
	return median(d)
}

// Movement is one source's change in its latest complete window.
type Movement struct {
	Current             float64  `json:"current"`
	BaselineMedian      *float64 `json:"baseline_median"`
	Lift                *float64 `json:"lift"`
	Z                   *float64 `json:"z"`
	History             int      `json:"history"`
	InsufficientHistory bool     `json:"insufficient_history"`
	PassesGate          bool     `json:"passes_gate"`
	DemandScore         float64  `json:"demand_score"`
}

func f(x float64) *float64 { return &x }

// CountMovement: lift = ln((c+1)/(b+1)); z = (log1p(c) − median(log1p(x))) / max(1.4826·MAD, 0.2).
func CountMovement(c float64, hist []float64, minCount, minZ float64) Movement {
	if len(hist) < minHistory {
		return Movement{Current: c, History: len(hist), InsufficientHistory: true}
	}
	b := median(hist)
	logs := make([]float64, len(hist))
	for i, x := range hist {
		logs[i] = math.Log1p(x)
	}
	z := (math.Log1p(c) - median(logs)) / math.Max(1.4826*mad(logs), 0.2)
	m := Movement{Current: c, BaselineMedian: f(b), Lift: f(math.Log((c + 1) / (b + 1))), Z: f(z), History: len(hist)}
	m.PassesGate = c >= minCount && z >= minZ
	if m.PassesGate {
		m.DemandScore = clamp(50 + 10*z)
	}
	return m
}

// IndexMovement: index feeds get no raw-count threshold (§5.3).
func IndexMovement(c float64, hist []float64, minZ float64) Movement {
	if len(hist) < minHistory {
		return Movement{Current: c, History: len(hist), InsufficientHistory: true}
	}
	b := median(hist)
	z := (c - b) / math.Max(1.4826*mad(hist), 1.0)
	m := Movement{Current: c, BaselineMedian: f(b), Z: f(z), History: len(hist), PassesGate: z >= minZ}
	if m.PassesGate {
		m.DemandScore = clamp(50 + 10*z)
	}
	return m
}

// OpportunityScore: 25% demand, 25% fit, 20% intent, 15% timing, 15% readiness.
func OpportunityScore(demand, fit, intent, timing, readiness float64) float64 {
	return 0.25*clamp(demand) + 0.25*clamp(fit) + 0.20*clamp(intent) + 0.15*clamp(timing) + 0.15*clamp(readiness)
}

// Confidence: 35% source quality, 25% corroboration, 20% coverage, 20% freshness; one family caps at 69.
func Confidence(quality, corroboration, coverage, freshness float64, families int) float64 {
	v := 0.35*clamp(quality) + 0.25*clamp(corroboration) + 0.20*clamp(coverage) + 0.20*clamp(freshness)
	if families <= 1 {
		return math.Min(v, singleFamilyCap)
	}
	return v
}

func Corroboration(families int) float64 {
	switch {
	case families <= 1:
		return 0
	case families == 2:
		return 80
	}
	return 100
}

// Freshness is 100 at window close, tapering linearly to 0 at the stale-after limit.
func Freshness(ageHours, staleAfter float64) float64 {
	if ageHours <= 0 {
		return 100
	}
	return clamp(100 * (1 - ageHours/staleAfter))
}

// Decide applies the §5 table. Bounded automatic action (≥ 85/85) is off: every live
// change needs an editor.
func Decide(score, confidence float64, vetoes []string, insufficient, shadow bool) (string, []string) {
	if score < 60 || confidence < 55 {
		return "archived", []string{"below the archive cut-offs (score 60, confidence 55)"}
	}
	var reasons []string
	if len(vetoes) > 0 {
		reasons = append(reasons, "veto")
	}
	if insufficient {
		reasons = append(reasons, "insufficient history: draft only")
	}
	if shadow {
		reasons = append(reasons, "profile in shadow mode")
	}
	if score >= 75 && confidence >= 75 && len(reasons) == 0 {
		return "eligible", []string{"reviewable brief"}
	}
	if len(reasons) == 0 {
		reasons = []string{"between the archive and brief thresholds: watch"}
	}
	return "watching", reasons
}
