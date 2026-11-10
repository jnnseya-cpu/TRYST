package growth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/staffauth"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

// Invisible by design (D-73): nothing here is shown to members or visitors as a feature.
// Members never see a screen, notice, label, cookie or script from this service; the only
// public effect is ordinary guide-page wording that an editor approved.

// Store kinds. Everything is a JSON document written through on every change.
const (
	kObs      = "growth_obs"
	kSnap     = "growth_snap"
	kBrief    = "growth_brief"
	kAction   = "growth_action"
	kPage     = "growth_page"
	kCount    = "growth_count"
	kAudit    = "growth_audit"
	kDedup    = "growth_dedup"
	kState    = "growth_state"
	stateMain = "main"
)

// Brief is a Growth Brief v1 with its review state. Body is the closed contract object.
type Brief struct {
	ID              string         `json:"id"`
	Status          string         `json:"status"` // draft | approved | superseded
	Body            map[string]any `json:"body"`
	SnapshotID      string         `json:"snapshot_id"`
	ProfileRevision string         `json:"profile_revision"`
	Author          string         `json:"author"`
	Approver        string         `json:"approver,omitempty"`
	Created         time.Time      `json:"created"`
	Approved        *time.Time     `json:"approved,omitempty"`
	Experiment      *Experiment    `json:"experiment,omitempty"`
}

func (b *Brief) str(k string) string { s, _ := b.Body[k].(string); return s }
func (b *Brief) version() int {
	switch v := b.Body["brief_version"].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}
func (b *Brief) strs(k string) []string {
	switch v := b.Body[k].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
func (b *Brief) expires() time.Time { t, _ := time.Parse(time.RFC3339, b.str("expires_at")); return t }
func (b *Brief) validFrom() time.Time {
	t, _ := time.Parse(time.RFC3339, b.str("valid_from"))
	return t
}

// Experiment is frozen when the brief is approved (§10.1).
type Experiment struct {
	ID            string    `json:"experiment_id"`
	Hypothesis    string    `json:"hypothesis"`
	Primary       string    `json:"primary_outcome"`
	ControlMethod string    `json:"control_method"`
	BaselineDays  int       `json:"baseline_days"`
	MinViews      int       `json:"min_views"`
	MinOutcomes   int       `json:"min_outcomes"`
	AssetIDs      []string  `json:"asset_ids"`
	Owner         string    `json:"owner"`
	FrozenAt      time.Time `json:"frozen_at"`
}

// Action is one typed change (live patch on a guide page) or an export-only draft.
type Action struct {
	ID               string            `json:"id"`
	BriefID          string            `json:"brief_id"`
	BriefVersion     int               `json:"brief_version"`
	Kind             string            `json:"action_kind"`
	Channel          string            `json:"channel"`
	TargetID         string            `json:"target_id"`
	ExpectedRevision int               `json:"expected_target_revision"`
	Patch            map[string]string `json:"patch"`
	EvidenceIDs      []string          `json:"evidence_ids"`
	ClaimIDs         []string          `json:"approved_claim_ids"`
	PayloadHash      string            `json:"payload_hash"`
	ActionKey        string            `json:"action_key"`
	PolicyVersion    int               `json:"policy_version"`
	State            string            `json:"state"`
	Live             bool              `json:"live"`
	Author           string            `json:"author"`
	Approval         *Approval         `json:"approval,omitempty"`
	Expires          time.Time         `json:"expires_at"`
	Previous         map[string]string `json:"previous_fields,omitempty"`
	AppliedRevision  int               `json:"applied_revision,omitempty"`
	AppliedAt        *time.Time        `json:"applied_at,omitempty"`
	Receipt          map[string]any    `json:"receipt,omitempty"`
	RollbackOf       string            `json:"rollback_of,omitempty"`
	LastError        string            `json:"last_error,omitempty"`
	Inference        map[string]any    `json:"inference,omitempty"`
	Created          time.Time         `json:"created"`
	Updated          time.Time         `json:"updated"`
}

type Approval struct {
	Approver       string    `json:"approver"`
	PayloadHash    string    `json:"payload_hash"`
	TargetRevision int       `json:"target_revision"`
	PolicyVersion  int       `json:"policy_version"`
	BriefVersion   int       `json:"brief_version"`
	At             time.Time `json:"at"`
}

// Page is a public guide page with immutable revisions; only Active is ever served.
type Page struct {
	AssetID   string         `json:"asset_id"`
	Slug      string         `json:"slug"`
	Active    int            `json:"active_revision"`
	Revisions []PageRevision `json:"revisions"`
	CreatedBy string         `json:"created_by"`
}

type PageRevision struct {
	Revision  int               `json:"revision"`
	Fields    map[string]string `json:"fields"`
	ActionKey string            `json:"action_key,omitempty"`
	At        time.Time         `json:"at"`
}

func (p *Page) current() PageRevision {
	for _, r := range p.Revisions {
		if r.Revision == p.Active {
			return r
		}
	}
	return PageRevision{}
}

type state struct {
	Frozen bool              `json:"frozen"`
	Active map[string]string `json:"active"` // market|locale|topic -> brief id
}

type auditEntry struct {
	Seq    int64          `json:"seq"`
	At     time.Time      `json:"at"`
	Actor  string         `json:"actor"`
	Event  string         `json:"event"`
	Entity string         `json:"entity"`
	Detail map[string]any `json:"detail,omitempty"`
}

// Signals is the member platform's aggregate feed (counts by city and week, k ≥ 50).
type Signals interface {
	Aggregates(ctx context.Context) ([]Observation, error)
}

// Service is growth-svc: console API, public guide pages, worker.
type Service struct {
	mu      sync.Mutex
	st      store.Store
	prof    *Profile
	staff   staffauth.Auth
	now     func() time.Time
	Signals Signals
	Gen     *Generator

	obs     map[string]*Observation
	snaps   map[string]*Snapshot
	briefs  map[string]*Brief
	actions map[string]*Action
	pages   map[string]*Page
	counts  map[string]int // asset|rev|day|kind
	dedup   map[string]dedupEntry
	state   state
	seq     int64
	audits  []auditEntry
}

type dedupEntry struct {
	Hash   string          `json:"hash"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

// New loads every document; the service keeps its state in memory and runs as one copy.
func New(ctx context.Context, st store.Store, prof *Profile, staff staffauth.Auth, now func() time.Time) (*Service, error) {
	if now == nil {
		now = time.Now
	}
	s := &Service{st: st, prof: prof, staff: staff, now: now, Gen: &Generator{},
		obs: map[string]*Observation{}, snaps: map[string]*Snapshot{}, briefs: map[string]*Brief{},
		actions: map[string]*Action{}, pages: map[string]*Page{}, counts: map[string]int{}, dedup: map[string]dedupEntry{},
		state: state{Active: map[string]string{}}}
	load := func(kind string, into func(id string, b []byte) error) error {
		all, err := st.All(ctx, kind)
		if err != nil {
			return err
		}
		for _, id := range store.Keys(all) {
			if err := into(id, all[id]); err != nil {
				return fmt.Errorf("%s %s: %w", kind, id, err)
			}
		}
		return nil
	}
	err := errors.Join(
		load(kObs, func(id string, b []byte) error {
			var v Observation
			err := json.Unmarshal(b, &v)
			s.obs[id] = &v
			return err
		}),
		load(kSnap, func(id string, b []byte) error {
			var v Snapshot
			err := json.Unmarshal(b, &v)
			s.snaps[id] = &v
			return err
		}),
		load(kBrief, func(id string, b []byte) error {
			var v Brief
			err := json.Unmarshal(b, &v)
			s.briefs[id] = &v
			return err
		}),
		load(kAction, func(id string, b []byte) error {
			var v Action
			err := json.Unmarshal(b, &v)
			s.actions[id] = &v
			return err
		}),
		load(kPage, func(id string, b []byte) error {
			var v Page
			err := json.Unmarshal(b, &v)
			s.pages[id] = &v
			return err
		}),
		load(kCount, func(id string, b []byte) error { var v int; err := json.Unmarshal(b, &v); s.counts[id] = v; return err }),
		load(kDedup, func(id string, b []byte) error {
			var v dedupEntry
			err := json.Unmarshal(b, &v)
			s.dedup[id] = v
			return err
		}),
		load(kAudit, func(id string, b []byte) error {
			var v auditEntry
			err := json.Unmarshal(b, &v)
			s.audits = append(s.audits, v)
			if v.Seq > s.seq {
				s.seq = v.Seq
			}
			return err
		}),
	)
	if err != nil {
		return nil, err
	}
	sort.Slice(s.audits, func(i, j int) bool { return s.audits[i].Seq < s.audits[j].Seq })
	if b, err := st.Get(ctx, kState, stateMain); err == nil {
		if err := json.Unmarshal(b, &s.state); err != nil {
			return nil, err
		}
		if s.state.Active == nil {
			s.state.Active = map[string]string{}
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	return s, nil
}

// Profile is the active profile.
func (s *Service) Profile() *Profile { return s.prof }

func newID(prefix string) string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// put writes a document through; failures are returned so the request fails visibly.
func (s *Service) put(ctx context.Context, kind, id string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.st.Put(ctx, kind, id, b)
}

func (s *Service) saveState(ctx context.Context) error { return s.put(ctx, kState, stateMain, s.state) }

// audit appends to the staff-only audit trail (caller holds s.mu).
func (s *Service) audit(ctx context.Context, actor, event, entity string, detail map[string]any) {
	s.seq++
	e := auditEntry{Seq: s.seq, At: s.now().UTC(), Actor: actor, Event: event, Entity: entity, Detail: detail}
	s.audits = append(s.audits, e)
	if err := s.put(ctx, kAudit, fmt.Sprintf("%012d", e.Seq), e); err != nil {
		log.Printf("growth audit: %v", err)
	}
}
