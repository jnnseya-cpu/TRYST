package growth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Observation is one aggregate count or index value for one source, term and window.
// It never carries a member identifier: the platform sends counts, staff import exports.
type Observation struct {
	SourceID       string    `json:"source_id"`
	SourceRecordID string    `json:"source_record_id"`
	RecordRevision int       `json:"record_revision"`
	Term           string    `json:"term"`
	Market         string    `json:"market"`
	Locale         string    `json:"locale"`
	Metric         string    `json:"metric"`
	Value          float64   `json:"value"`
	Unit           string    `json:"unit"`
	Coverage       float64   `json:"coverage"`
	WindowStart    time.Time `json:"window_start"`
	WindowEnd      time.Time `json:"window_end"`
	ObservedAt     time.Time `json:"observed_at"`
	EvidenceHash   string    `json:"evidence_hash,omitempty"`
	EvidenceURI    string    `json:"evidence_uri,omitempty"`

	// Set on ingest.
	ID          string    `json:"id,omitempty"`
	Family      string    `json:"family,omitempty"`
	TopicID     string    `json:"topic_id,omitempty"`
	ContentHash string    `json:"content_hash,omitempty"`
	IngestedAt  time.Time `json:"ingested_at,omitempty"`
}

func (o Observation) key() string {
	return fmt.Sprintf("%s|%s|%d", o.SourceID, o.SourceRecordID, o.RecordRevision)
}

func digest(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

var nonWord = regexp.MustCompile(`[^\pL\pN]+`)

// CanonicalTerm folds case, accents and punctuation; the original text is kept too.
func CanonicalTerm(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(strings.ToLower(s)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(nonWord.ReplaceAllString(b.String(), " "))
}

// TopicFor maps a term to the first topic whose label or an alias it contains word for word.
func (p *Profile) TopicFor(term string) string {
	words := map[string]bool{}
	for _, w := range strings.Fields(CanonicalTerm(term)) {
		words[w] = true
	}
	for _, t := range p.Topics {
		for _, alias := range append([]string{t.Label}, t.Aliases...) {
			aw := strings.Fields(CanonicalTerm(alias))
			all := len(aw) > 0
			for _, w := range aw {
				all = all && words[w]
			}
			if all {
				return t.ID
			}
		}
	}
	slug := strings.ReplaceAll(CanonicalTerm(term), " ", "-")
	if len(slug) > 100 {
		slug = slug[:100]
	}
	if slug == "" {
		slug = "unknown"
	}
	return "unmapped-" + slug
}

// SourceDetail explains one source's contribution to a snapshot.
type SourceDetail struct {
	Movement
	Family      string   `json:"family"`
	Kind        string   `json:"kind"`
	ContextOnly bool     `json:"context_only"`
	Quality     float64  `json:"quality"`
	WindowEnd   string   `json:"window_end"`
	AgeHours    float64  `json:"age_hours"`
	Stale       bool     `json:"stale"`
	Freshness   float64  `json:"freshness"`
	Coverage    float64  `json:"coverage"`
	EvidenceIDs []string `json:"evidence_ids"`
	hashes      map[string]bool
}

// Snapshot is a frozen score for one (topic, market, locale), keyed by its input digest.
type Snapshot struct {
	ID              string                   `json:"id"`
	TopicID         string                   `json:"topic_id"`
	Market          string                   `json:"market"`
	Locale          string                   `json:"locale"`
	WindowEnd       time.Time                `json:"window_end"`
	ScoringVersion  string                   `json:"scoring_version"`
	InputDigest     string                   `json:"input_digest"`
	ProfileRevision string                   `json:"profile_revision"`
	Score           float64                  `json:"score"`
	Confidence      float64                  `json:"confidence"`
	Status          string                   `json:"status"`
	TrendClass      string                   `json:"trend_class"`
	Components      map[string]float64       `json:"components"`
	ConfidenceIn    map[string]float64       `json:"confidence_inputs"`
	Families        [][]string               `json:"families"`
	Sources         map[string]*SourceDetail `json:"sources"`
	Vetoes          []string                 `json:"vetoes"`
	Reasons         []string                 `json:"reasons"`
	Notes           []string                 `json:"notes"`
	EvidenceIDs     []string                 `json:"evidence_ids"`
	Insufficient    bool                     `json:"insufficient_history"`
	CreatedAt       time.Time                `json:"created_at"`
}

var horizonHours = map[string]float64{"breaking": 24, "persistent": 168, "seasonal": 168}

type window struct {
	start, end time.Time
	value      float64
	obs        []*Observation
}

// ScoreScope scores one (topic, market, locale) from its observations at time now.
func ScoreScope(p *Profile, obs []*Observation, topicID, market, locale string, now time.Time) *Snapshot {
	bySource := map[string]map[[2]time.Time]*window{}
	for _, o := range obs {
		if bySource[o.SourceID] == nil {
			bySource[o.SourceID] = map[[2]time.Time]*window{}
		}
		k := [2]time.Time{o.WindowStart, o.WindowEnd}
		w := bySource[o.SourceID][k]
		if w == nil {
			w = &window{start: o.WindowStart, end: o.WindowEnd}
			bySource[o.SourceID][k] = w
		}
		w.obs = append(w.obs, o)
	}
	snap := &Snapshot{TopicID: topicID, Market: market, Locale: locale, Sources: map[string]*SourceDetail{},
		Vetoes: []string{}, Notes: []string{}, EvidenceIDs: []string{}, Families: [][]string{}}
	considered := map[string]*SourceDetail{}
	insufficientAll := true
	ids := make([]string, 0, len(bySource))
	for id := range bySource {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, sid := range ids {
		src := p.Source(sid)
		if src == nil {
			continue
		}
		var complete []*window
		for _, w := range bySource[sid] {
			if !w.end.After(now) {
				sum := 0.0
				for _, o := range w.obs {
					sum += o.Value
				}
				if src.Kind == "index" {
					sum /= float64(len(w.obs))
				}
				w.value = sum
				complete = append(complete, w)
			}
		}
		if len(complete) == 0 {
			continue
		}
		sort.Slice(complete, func(i, j int) bool { return complete[i].end.Before(complete[j].end) })
		cur := complete[len(complete)-1]
		span := cur.end.Sub(cur.start)
		var hist []float64
		for _, w := range complete[:len(complete)-1] {
			if w.end.Sub(w.start) == span {
				hist = append(hist, w.value)
			}
		}
		if len(hist) > 8 {
			hist = hist[len(hist)-8:]
		}
		minCount, minZ := src.MinCount, src.MinZ
		if minCount == 0 {
			minCount = 20
		}
		if minZ == 0 {
			minZ = 2
		}
		var mv Movement
		if src.Kind == "count" {
			mv = CountMovement(cur.value, hist, minCount, minZ)
		} else {
			mv = IndexMovement(cur.value, hist, minZ)
		}
		age := now.Sub(cur.end).Hours()
		d := &SourceDetail{Movement: mv, Family: src.Family, Kind: src.Kind, ContextOnly: src.ContextOnly,
			Quality: src.Quality, WindowEnd: cur.end.UTC().Format(time.RFC3339), AgeHours: age,
			Stale: age > src.StaleAfterHours, Freshness: Freshness(age, src.StaleAfterHours), hashes: map[string]bool{}}
		cov := 0.0
		for _, o := range cur.obs {
			cov += o.Coverage
			d.EvidenceIDs = append(d.EvidenceIDs, o.ID)
			if o.EvidenceHash != "" {
				d.hashes[o.EvidenceHash] = true
			}
		}
		d.Coverage = cov / float64(len(cur.obs))
		sort.Strings(d.EvidenceIDs)
		snap.Sources[sid] = d
		insufficientAll = insufficientAll && mv.InsufficientHistory
		if snap.WindowEnd.Before(cur.end) {
			snap.WindowEnd = cur.end
		}
		switch {
		case d.Stale:
			snap.Notes = append(snap.Notes, fmt.Sprintf("%s: stale (%.0f h old); dated context only", sid, age))
		case src.ContextOnly:
			snap.Notes = append(snap.Notes, sid+": context only; cannot corroborate")
		case mv.InsufficientHistory:
			snap.Notes = append(snap.Notes, fmt.Sprintf("%s: insufficient history (%d comparable windows)", sid, mv.History))
		case mv.PassesGate:
			considered[sid] = d
		}
	}
	snap.Families = families(considered)
	var reps []string
	for _, fam := range snap.Families {
		best := append([]string(nil), fam...)
		sort.Slice(best, func(i, j int) bool {
			a, b := snap.Sources[best[i]], snap.Sources[best[j]]
			if a.DemandScore != b.DemandScore {
				return a.DemandScore > b.DemandScore
			}
			return best[i] < best[j]
		})
		reps = append(reps, best[0])
	}
	topic := p.Topic(topicID)
	var demand, fit, intent, timing, readiness float64
	if len(reps) > 0 {
		var ds []float64
		minAge := -1.0
		for _, r := range reps {
			ds = append(ds, snap.Sources[r].DemandScore)
			if minAge < 0 || snap.Sources[r].AgeHours < minAge {
				minAge = snap.Sources[r].AgeHours
			}
		}
		demand = median(ds)
		snap.TrendClass = "persistent"
		if topic != nil && topic.TrendClass != "" {
			snap.TrendClass = topic.TrendClass
		}
		timing = Freshness(minAge, horizonHours[snap.TrendClass])
	}
	if snap.TrendClass == "" {
		snap.TrendClass = "persistent"
	}
	if topic != nil {
		if m, ok := topic.Markets[market]; ok {
			fit, intent = m.Fit, m.Intent
		}
		readiness = topic.Readiness
	}
	if market != p.DefaultMarket {
		snap.Vetoes = append(snap.Vetoes, "out_of_market")
	}
	if locale != p.DefaultLocale {
		snap.Vetoes = append(snap.Vetoes, "unsupported_locale")
	}
	if topic == nil {
		snap.Vetoes = append(snap.Vetoes, "unmapped_topic")
	} else {
		if topic.Restricted {
			snap.Vetoes = append(snap.Vetoes, "restricted_topic")
		}
		if len(topic.ClaimIDs) == 0 {
			snap.Vetoes = append(snap.Vetoes, "unsupported_claims")
		}
		if readiness <= 0 {
			snap.Vetoes = append(snap.Vetoes, "not_ready")
		}
		if len(topic.AssetIDs) == 0 || topic.CTAID == "" {
			snap.Vetoes = append(snap.Vetoes, "no_approved_page_or_cta")
		}
	}
	snap.Score = OpportunityScore(demand, fit, intent, timing, readiness)
	var quality, coverage, fresh float64
	for _, r := range reps {
		quality += snap.Sources[r].Quality
		coverage += 100 * snap.Sources[r].Coverage
		fresh += snap.Sources[r].Freshness
	}
	if n := float64(len(reps)); n > 0 {
		quality, coverage, fresh = quality/n, coverage/n, fresh/n
		snap.Confidence = Confidence(quality, Corroboration(len(reps)), coverage, fresh, len(reps))
	}
	snap.Components = map[string]float64{"demand": demand, "fit": fit, "intent": intent, "timing": timing, "readiness": readiness}
	snap.ConfidenceIn = map[string]float64{"quality": quality, "corroboration": Corroboration(len(reps)),
		"coverage": coverage, "freshness": fresh, "families": float64(len(reps))}
	snap.Insufficient = insufficientAll
	snap.Status, snap.Reasons = Decide(snap.Score, snap.Confidence, snap.Vetoes, insufficientAll, p.Mode == "shadow")
	ev := map[string]bool{}
	for _, r := range reps {
		for _, e := range snap.Sources[r].EvidenceIDs {
			ev[e] = true
		}
	}
	for e := range ev {
		snap.EvidenceIDs = append(snap.EvidenceIDs, e)
	}
	sort.Strings(snap.EvidenceIDs)
	if len(snap.EvidenceIDs) > 64 {
		snap.EvidenceIDs = snap.EvidenceIDs[:64]
	}
	return snap
}

// families unions sources that share a declared family or any evidence hash (syndication,
// mirrors, a partner export of the same data): each group counts once.
func families(sources map[string]*SourceDetail) [][]string {
	ids := make([]string, 0, len(sources))
	for id := range sources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	parent := map[string]string{}
	for _, id := range ids {
		parent[id] = id
	}
	find := func(x string) string {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	seen := map[string]string{}
	for _, id := range ids {
		keys := []string{"family:" + sources[id].Family}
		for h := range sources[id].hashes {
			keys = append(keys, "hash:"+h)
		}
		for _, k := range keys {
			if other, ok := seen[k]; ok {
				parent[find(id)] = find(other)
			} else {
				seen[k] = id
			}
		}
	}
	groups := map[string][]string{}
	for _, id := range ids {
		r := find(id)
		groups[r] = append(groups[r], id)
	}
	out := make([][]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}
