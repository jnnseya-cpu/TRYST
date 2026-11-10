package growth

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// Err is a typed error for the console API: {code, message, details}.
type Err struct {
	Status  int
	Code    string
	Msg     string
	Details []string
}

func (e *Err) Error() string { return e.Code + ": " + e.Msg }

func bad(msg string, details ...string) *Err { return &Err{422, "validation_failed", msg, details} }
func conflict(code, msg string, details ...string) *Err {
	return &Err{409, code, msg, details}
}
func notFound() *Err { return &Err{404, "not_found", "Not found", nil} }

var (
	emailRe  = regexp.MustCompile(`[^\s@]+@[^\s@]+\.[^\s@]+`)
	phoneRe  = regexp.MustCompile(`\+?\d[\d\s().-]{7,}\d`)
	marketRe = regexp.MustCompile(`^[A-Z]{2}$`)
)

// ---------------------------------------------------------------- ingestion

// Ingest stores a batch atomically: any invalid item rejects the whole batch with indexed
// errors; an exact replay is a duplicate; the same source version with different content
// is a conflict and nothing is stored.
func (s *Service) Ingest(ctx context.Context, items []Observation, actor string) (map[string]int, error) {
	return s.ingest(ctx, items, actor, false)
}

// ingest: staff may import only file_import sources; the member platform's counts arrive
// only from the platform itself (fromPlatform), so nobody can type them in.
func (s *Service) ingest(ctx context.Context, items []Observation, actor string, fromPlatform bool) (map[string]int, error) {
	if len(items) == 0 || len(items) > 500 {
		return nil, bad("Send 1 to 500 observations.")
	}
	now := s.now()
	var errs []string
	for i := range items {
		o := &items[i]
		if o.RecordRevision == 0 {
			o.RecordRevision = 1
		}
		src := s.prof.Source(o.SourceID)
		switch {
		case src == nil:
			errs = append(errs, fmt.Sprintf("[%d].source_id: not a registered source", i))
		case src.Kind != o.Unit:
			errs = append(errs, fmt.Sprintf("[%d].unit: %s reports %s", i, o.SourceID, src.Kind))
		case fromPlatform != (src.AccessPath == "first_party"):
			errs = append(errs, fmt.Sprintf("[%d].source_id: %s can't be imported here", i, o.SourceID))
		}
		if !idRe.MatchString(o.SourceRecordID) {
			errs = append(errs, fmt.Sprintf("[%d].source_record_id: letters, digits, - and _ only", i))
		}
		if o.Term == "" || len(o.Term) > 300 || emailRe.MatchString(o.Term) || phoneRe.MatchString(o.Term) {
			errs = append(errs, fmt.Sprintf("[%d].term: 1-300 characters of aggregated topic text, no personal data", i))
		}
		if !marketRe.MatchString(o.Market) || o.Locale == "" || o.Value < 0 || math.IsNaN(o.Value) || o.Coverage < 0 || o.Coverage > 1 {
			errs = append(errs, fmt.Sprintf("[%d]: market, locale, value ≥ 0 and coverage 0-1 are required", i))
		}
		if !o.WindowStart.Before(o.WindowEnd) {
			errs = append(errs, fmt.Sprintf("[%d]: window_start must precede window_end (end is exclusive)", i))
		}
		if o.WindowEnd.After(now.Add(5*time.Minute)) || o.ObservedAt.After(now.Add(5*time.Minute)) {
			errs = append(errs, fmt.Sprintf("[%d]: times are in the future", i))
		}
	}
	if len(errs) > 0 {
		return nil, bad("Batch rejected; nothing was stored.", errs...)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	accepted, dup := 0, 0
	var conflicts []string
	var fresh []*Observation
	for i := range items {
		o := items[i]
		o.ID, o.Family, o.TopicID, o.IngestedAt = "", "", "", time.Time{}
		h := digest(o)
		if old, ok := s.obs[o.key()]; ok {
			if old.ContentHash == h {
				dup++
			} else {
				conflicts = append(conflicts, fmt.Sprintf("[%d]: %s already stored with different content; send a new record_revision", i, o.key()))
			}
			continue
		}
		o.ContentHash, o.ID = h, newID("ob_")
		o.Family, o.TopicID, o.IngestedAt = s.prof.Source(o.SourceID).Family, s.prof.TopicFor(o.Term), now.UTC()
		fresh = append(fresh, &o)
	}
	if len(conflicts) > 0 {
		return nil, conflict("observation_conflict", "Same source version with different content; nothing was stored.", conflicts...)
	}
	for _, o := range fresh {
		if err := s.put(ctx, kObs, o.key(), o); err != nil {
			return nil, err
		}
		s.obs[o.key()] = o
		accepted++
	}
	s.audit(ctx, actor, "observations.ingested", "observations", map[string]any{"accepted": accepted, "duplicate": dup})
	return map[string]int{"accepted": accepted, "duplicate": dup, "rejected": 0}, nil
}

// ---------------------------------------------------------------- scoring

// Refresh pulls the platform's aggregates (if reachable), then scores every scope.
func (s *Service) Refresh(ctx context.Context, actor string) (map[string]any, error) {
	pulled := -1
	if s.Signals != nil {
		obs, err := s.Signals.Aggregates(ctx)
		if err != nil {
			pulled = -2
		} else if len(obs) > 0 {
			res, err := s.ingest(ctx, obs, "platform", true)
			if err != nil {
				return nil, err
			}
			pulled = res["accepted"]
		} else {
			pulled = 0
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	scopes := map[[3]string][]*Observation{}
	since := now.Add(-11 * 7 * 24 * time.Hour)
	for _, o := range s.obs {
		if o.WindowEnd.After(since) {
			k := [3]string{o.TopicID, o.Market, o.Locale}
			scopes[k] = append(scopes[k], o)
		}
	}
	eligible, scored := 0, 0
	for k, rows := range scopes {
		snap := ScoreScope(s.prof, rows, k[0], k[1], k[2], now)
		if snap.WindowEnd.IsZero() {
			continue
		}
		keys := make([]string, 0, len(rows))
		for _, o := range rows {
			keys = append(keys, o.key()+"|"+o.ContentHash)
		}
		sort.Strings(keys)
		snap.InputDigest = digest([]any{s.prof.Revision, now.UTC().Format("2006-01-02T15"), keys})
		snap.ScoringVersion, snap.ProfileRevision, snap.CreatedAt = ScoringVersion, s.prof.Revision, now.UTC()
		snap.ID = "op_" + digest([]string{k[0], k[1], k[2], snap.WindowEnd.String(), ScoringVersion, snap.InputDigest})[:20]
		if _, exists := s.snaps[snap.ID]; !exists {
			if err := s.put(ctx, kSnap, snap.ID, snap); err != nil {
				return nil, err
			}
			s.snaps[snap.ID] = snap
		}
		scored++
		if snap.Status == "eligible" {
			eligible++
		}
	}
	s.audit(ctx, actor, "scoring.run", "opportunities", map[string]any{"scopes": scored, "eligible": eligible, "platform_rows": pulled})
	out := map[string]any{"scored": scored, "eligible": eligible}
	switch pulled {
	case -1:
		out["platform"] = "not connected"
	case -2:
		out["platform"] = "unreachable; scored on stored data"
	default:
		out["platform_rows"] = pulled
	}
	return out, nil
}

// Opportunities returns the latest snapshot per scope, best first.
func (s *Service) Opportunities(status string) []*Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	latest := map[string]*Snapshot{}
	for _, sn := range s.snaps {
		k := sn.TopicID + "|" + sn.Market + "|" + sn.Locale
		if cur := latest[k]; cur == nil || sn.CreatedAt.After(cur.CreatedAt) || (sn.CreatedAt.Equal(cur.CreatedAt) && sn.ID > cur.ID) {
			latest[k] = sn
		}
	}
	out := []*Snapshot{}
	for _, sn := range latest {
		if status == "" || sn.Status == status {
			out = append(out, sn)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// ---------------------------------------------------------------- briefs

var validity = map[string]time.Duration{"breaking": 24 * time.Hour, "persistent": 7 * 24 * time.Hour, "seasonal": 7 * 24 * time.Hour}

func (s *Service) CreateBrief(ctx context.Context, snapshotID, actor string) (*Brief, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.snaps[snapshotID]
	if snap == nil {
		return nil, notFound()
	}
	if snap.Status != "eligible" {
		return nil, conflict("insufficient_evidence", "This opportunity is "+snap.Status+", not eligible for a brief.", snap.Reasons...)
	}
	if snap.ProfileRevision != s.prof.Revision {
		return nil, conflict("stale_snapshot", "The profile changed after scoring; refresh first.")
	}
	topic := s.prof.Topic(snap.TopicID)
	var channels []string
	for _, c := range s.prof.AllowedChannels {
		if slices.Contains(EnabledChannels, c) {
			channels = append(channels, c)
		}
	}
	briefID := "gb_" + digest([]string{snap.Market, snap.Locale, snap.TopicID})[:16]
	version := 0
	for _, b := range s.briefs {
		if b.str("brief_id") == briefID && b.version() > version {
			version = b.version()
		}
	}
	now := s.now().UTC().Truncate(time.Second)
	body := map[string]any{
		"schema_version": 1, "brief_id": briefID, "brief_version": version + 1,
		"tenant_id": s.prof.PlatformSlug, "platform_id": s.prof.PlatformSlug,
		"market": snap.Market, "locale": snap.Locale, "topic_id": snap.TopicID,
		"opportunity_score": snap.Score, "confidence": snap.Confidence,
		"valid_from": now.Format(time.RFC3339), "expires_at": now.Add(validity[snap.TrendClass]).Format(time.RFC3339),
		"evidence_ids": snap.EvidenceIDs, "audience": topic.Audience, "intent": topic.Intent,
		"approved_claim_ids": topic.ClaimIDs, "cta_id": topic.CTAID, "allowed_channels": channels,
		"prohibited_claims": s.prof.ProhibitedClaims, "objective": s.prof.Objective,
		"experiment_id": newID("exp_")[:16], "policy_version": s.prof.PolicyVersion,
	}
	if errs := BriefSchemaErrors(body); len(errs) > 0 {
		return nil, bad("The brief would not satisfy the Growth Brief v1 contract.", errs...)
	}
	b := &Brief{ID: newID("br_"), Status: "draft", Body: body, SnapshotID: snap.ID, ProfileRevision: s.prof.Revision,
		Author: actor, Created: now}
	if err := s.put(ctx, kBrief, b.ID, b); err != nil {
		return nil, err
	}
	s.briefs[b.ID] = b
	s.audit(ctx, actor, "brief.drafted", "brief:"+briefID+":"+fmt.Sprint(version+1),
		map[string]any{"snapshot": snap.ID, "score": snap.Score, "confidence": snap.Confidence})
	return b, nil
}

func scopeKey(market, locale, topic string) string { return market + "|" + locale + "|" + topic }

func (s *Service) ApproveBrief(ctx context.Context, id, ifMatch, actor string) (*Brief, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.briefs[id]
	if b == nil {
		return nil, notFound()
	}
	if ifMatch == "" {
		return nil, &Err{428, "precondition_required", "Approve an exact brief version with If-Match.", nil}
	}
	if strings.Trim(ifMatch, `"`) != fmt.Sprint(b.version()) {
		return nil, conflict("revision_mismatch", fmt.Sprintf("The brief is at version %d.", b.version()))
	}
	if b.Status != "draft" {
		return nil, conflict("invalid_state", "The brief is "+b.Status+".")
	}
	if b.Author == actor {
		return nil, &Err{403, "self_approval", "The author of a brief can't approve it.", nil}
	}
	errs := append(BriefSchemaErrors(b.Body), s.runtimeErrors(b, "")...)
	if b.ProfileRevision != s.prof.Revision {
		errs = append(errs, "the profile changed since the brief was drafted")
	}
	if len(errs) > 0 {
		return nil, conflict("brief_invalid", "The brief can no longer be approved.", errs...)
	}
	now := s.now().UTC()
	key := scopeKey(b.str("market"), b.str("locale"), b.str("topic_id"))
	if prev := s.briefs[s.state.Active[key]]; prev != nil {
		prev.Status = "superseded"
		if err := s.put(ctx, kBrief, prev.ID, prev); err != nil {
			return nil, err
		}
	}
	topic := s.prof.Topic(b.str("topic_id"))
	b.Status, b.Approver, b.Approved = "approved", actor, &now
	b.Experiment = &Experiment{ID: b.str("experiment_id"),
		Hypothesis: fmt.Sprintf("Applying this brief's approved changes to %s raises %s per %s.",
			strings.Join(topic.AssetIDs, ", "), s.prof.Outcome.Conversion, s.prof.Outcome.Denominator),
		Primary: s.prof.Outcome.Conversion, ControlMethod: "pre_post_windows_observational", BaselineDays: 14,
		MinViews: 200, MinOutcomes: 10, AssetIDs: topic.AssetIDs, Owner: actor, FrozenAt: now}
	s.state.Active[key] = b.ID
	if err := s.put(ctx, kBrief, b.ID, b); err != nil {
		return nil, err
	}
	if err := s.saveState(ctx); err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "brief.approved", "brief:"+b.str("brief_id")+":"+fmt.Sprint(b.version()),
		map[string]any{"author": b.Author, "experiment": b.Experiment.ID})
	return b, nil
}

// runtimeErrors applies the rules beyond the schema (Appendix A). Caller holds s.mu.
func (s *Service) runtimeErrors(b *Brief, channel string) []string {
	var errs []string
	vf, ex, now := b.validFrom(), b.expires(), s.now()
	if !vf.Before(ex) {
		errs = append(errs, "valid_from must precede expires_at")
	}
	if now.Before(vf) || !now.Before(ex) {
		errs = append(errs, "the brief is outside its validity window")
	}
	if channel != "" && !slices.Contains(b.strs("allowed_channels"), channel) {
		errs = append(errs, "channel "+channel+" is not allowed by this brief")
	}
	pv, _ := b.Body["policy_version"].(float64)
	if iv, ok := b.Body["policy_version"].(int); ok {
		pv = float64(iv)
	}
	if int(pv) != s.prof.PolicyVersion {
		errs = append(errs, "policy version does not match the active profile")
	}
	if b.str("market") != s.prof.DefaultMarket || b.str("locale") != s.prof.DefaultLocale {
		errs = append(errs, "market or locale is not installed")
	}
	return errs
}

// Effective returns the one approved, fresh brief for a scope and channel, or nil (baseline).
func (s *Service) Effective(market, locale, topic, channel string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.briefs[s.state.Active[scopeKey(market, locale, topic)]]
	if b == nil || b.Status != "approved" || len(s.runtimeErrors(b, channel)) > 0 {
		return nil
	}
	return b.Body
}

// activeBrief resolves the active, approved, unexpired version. Caller holds s.mu.
func (s *Service) activeBrief(briefID string, version int) (*Brief, error) {
	for _, b := range s.briefs {
		if b.str("brief_id") != briefID || b.version() != version {
			continue
		}
		key := scopeKey(b.str("market"), b.str("locale"), b.str("topic_id"))
		if b.Status != "approved" || s.state.Active[key] != b.ID {
			return nil, conflict("brief_not_active", "Only the active approved brief version can drive a change.")
		}
		if !s.now().Before(b.expires()) {
			return nil, conflict("brief_expired", "The brief has expired.")
		}
		return b, nil
	}
	return nil, notFound()
}

// ---------------------------------------------------------------- actions

// Kinds: live patches on guide pages, and export-only drafts.
var kinds = map[string]struct {
	channel string
	fields  []string
	live    bool
}{
	"seo_metadata_patch":   {"seo", []string{"title", "description"}, true},
	"owned_content_patch":  {"owned_placement", []string{"headline", "cta_text"}, true},
	"organic_social_draft": {"organic_social", []string{"post"}, false},
}

var markupRe = regexp.MustCompile(`(?i)<[a-z/!]|javascript:|https?://`)

// Proposal is a requested action.
type Proposal struct {
	BriefID          string            `json:"brief_id"`
	BriefVersion     int               `json:"brief_version"`
	Kind             string            `json:"action_kind"`
	Channel          string            `json:"channel"`
	TargetID         string            `json:"target_id"`
	ExpectedRevision int               `json:"expected_target_revision"`
	Patch            map[string]string `json:"patch"`
	EvidenceIDs      []string          `json:"evidence_ids"`
	ClaimIDs         []string          `json:"approved_claim_ids"`
}

func (s *Service) Propose(ctx context.Context, p Proposal, actor string, inference map[string]any) (*Action, bool, error) {
	k, ok := kinds[p.Kind]
	if !ok {
		return nil, false, bad("Unknown action kind.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Frozen {
		return nil, false, conflict("frozen", "Growth changes are frozen.")
	}
	b, err := s.activeBrief(p.BriefID, p.BriefVersion)
	if err != nil {
		return nil, false, err
	}
	var errs []string
	if p.Channel != k.channel {
		errs = append(errs, p.Kind+" belongs to channel "+k.channel)
	}
	if !slices.Contains(b.strs("allowed_channels"), p.Channel) {
		errs = append(errs, "channel "+p.Channel+" is not allowed by the brief")
	}
	if len(p.Patch) == 0 {
		errs = append(errs, "patch is empty")
	}
	text := ""
	for f, v := range p.Patch {
		lim, ok := FieldLimits[f]
		switch {
		case !ok || !slices.Contains(k.fields, f):
			errs = append(errs, "field "+f+" can't be changed by "+p.Kind)
		case strings.TrimSpace(v) == "" || len([]rune(v)) > lim:
			errs = append(errs, fmt.Sprintf("%s must be 1-%d characters", f, lim))
		case markupRe.MatchString(v):
			errs = append(errs, f+" contains markup or a link")
		}
		text += " " + v
	}
	if w := s.prof.Prohibited(text); w != "" {
		errs = append(errs, "prohibited wording: "+w)
	}
	for _, c := range p.ClaimIDs {
		if !slices.Contains(b.strs("approved_claim_ids"), c) {
			errs = append(errs, "claim "+c+" is not approved by the brief")
		}
	}
	for _, e := range p.EvidenceIDs {
		if !slices.Contains(b.strs("evidence_ids"), e) {
			errs = append(errs, "evidence "+e+" is not in the brief")
		}
	}
	if len(errs) > 0 {
		sort.Strings(errs)
		return nil, false, bad("The change failed validation.", errs...)
	}
	if len(p.EvidenceIDs) == 0 {
		return nil, false, conflict("insufficient_evidence", "A change must cite the brief's evidence.")
	}
	topic := s.prof.Topic(b.str("topic_id"))
	asset := s.prof.Asset(p.TargetID)
	if asset == nil || !slices.Contains(topic.AssetIDs, p.TargetID) {
		return nil, false, notFound()
	}
	if k.live {
		for f := range p.Patch {
			if !slices.Contains(asset.AllowedFields, f) {
				return nil, false, bad("Field not allowed on this page.", f)
			}
		}
		page := s.pages[p.TargetID]
		if page == nil {
			return nil, false, conflict("page_not_created", "An editor must create this page before changes can be proposed.")
		}
		if page.Active != p.ExpectedRevision {
			return nil, false, conflict("target_revision_conflict", fmt.Sprintf("The page is at revision %d.", page.Active))
		}
	}
	sort.Strings(p.EvidenceIDs)
	sort.Strings(p.ClaimIDs)
	payload := digest(p)
	key := "ak_" + digest([]any{p.BriefID, p.BriefVersion, p.Channel, p.Kind, p.TargetID, p.ExpectedRevision, digest(p.Patch)})[:40]
	for _, a := range s.actions {
		if a.ActionKey == key {
			return a, false, nil
		}
	}
	now := s.now().UTC()
	a := &Action{ID: newID("ac_"), BriefID: p.BriefID, BriefVersion: p.BriefVersion, Kind: p.Kind, Channel: p.Channel,
		TargetID: p.TargetID, ExpectedRevision: p.ExpectedRevision, Patch: p.Patch, EvidenceIDs: p.EvidenceIDs,
		ClaimIDs: p.ClaimIDs, PayloadHash: payload, ActionKey: key, PolicyVersion: s.prof.PolicyVersion,
		State: "validated", Live: k.live, Author: actor, Expires: b.expires(), Inference: inference, Created: now, Updated: now}
	if k.live {
		a.State = "pending_approval"
	}
	if err := s.put(ctx, kAction, a.ID, a); err != nil {
		return nil, false, err
	}
	s.actions[a.ID] = a
	s.audit(ctx, actor, "action.proposed", "action:"+a.ID, map[string]any{"kind": a.Kind, "target": a.TargetID, "payload_hash": payload})
	return a, true, nil
}

func (s *Service) ApproveAction(ctx context.Context, id, payloadHash string, targetRevision int, actor string) (*Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.actions[id]
	if a == nil {
		return nil, notFound()
	}
	if a.State != "pending_approval" {
		return nil, conflict("invalid_state", "The change is "+a.State+".")
	}
	if a.Author == actor {
		return nil, &Err{403, "self_approval", "The author of a change can't approve it.", nil}
	}
	if payloadHash != a.PayloadHash {
		return nil, conflict("payload_mismatch", "Approve the exact change shown in the preview.")
	}
	if targetRevision != a.ExpectedRevision {
		return nil, conflict("target_revision_conflict", "Approve against the previewed page revision.")
	}
	if _, err := s.activeBrief(a.BriefID, a.BriefVersion); err != nil {
		return nil, err
	}
	if a.PolicyVersion != s.prof.PolicyVersion {
		return nil, conflict("policy_changed", "Policy changed after the change was proposed.")
	}
	now := s.now().UTC()
	a.Approval = &Approval{Approver: actor, PayloadHash: payloadHash, TargetRevision: targetRevision,
		PolicyVersion: a.PolicyVersion, BriefVersion: a.BriefVersion, At: now}
	a.State, a.Updated = "approved", now
	if err := s.put(ctx, kAction, a.ID, a); err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "action.approved", "action:"+a.ID, map[string]any{"payload_hash": payloadHash})
	return a, nil
}

// Queue moves an approved change to the worker (the outbox is the action's own state).
func (s *Service) Queue(ctx context.Context, id, actor string) (*Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.actions[id]
	if a == nil {
		return nil, notFound()
	}
	if a.State != "approved" {
		return nil, conflict("invalid_state", "The change is "+a.State+".")
	}
	if s.state.Frozen {
		return nil, conflict("frozen", "Growth changes are frozen.")
	}
	a.State, a.Updated = "queued", s.now().UTC()
	if err := s.put(ctx, kAction, a.ID, a); err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "action.queued", "action:"+a.ID, nil)
	return a, nil
}

// Rollback queues a compensating change; it is allowed while frozen (§18.6).
func (s *Service) Rollback(ctx context.Context, id, actor string) (*Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.actions[id]
	if a == nil {
		return nil, notFound()
	}
	if a.State != "applied" || !a.Live || a.RollbackOf != "" {
		return nil, conflict("invalid_state", "Only an applied live change can be rolled back.")
	}
	key := "rb_" + digest([]string{"rollback", a.ActionKey})[:40]
	for _, x := range s.actions {
		if x.ActionKey == key {
			return x, nil
		}
	}
	now := s.now().UTC()
	r := &Action{ID: newID("ac_"), BriefID: a.BriefID, BriefVersion: a.BriefVersion, Kind: a.Kind, Channel: a.Channel,
		TargetID: a.TargetID, ExpectedRevision: a.AppliedRevision, Patch: a.Previous, EvidenceIDs: a.EvidenceIDs,
		ClaimIDs: a.ClaimIDs, PayloadHash: digest(a.Previous), ActionKey: key, PolicyVersion: a.PolicyVersion,
		State: "queued", Live: true, Author: actor, Expires: now.Add(30 * 24 * time.Hour), RollbackOf: a.ID,
		Created: now, Updated: now}
	if err := s.put(ctx, kAction, r.ID, r); err != nil {
		return nil, err
	}
	s.actions[r.ID] = r
	s.audit(ctx, actor, "action.rollback_queued", "action:"+r.ID, map[string]any{"rollback_of": a.ID})
	return r, nil
}

// ---------------------------------------------------------------- worker

// Work runs every queued change once and settles any interrupted one by its receipt.
// It returns what happened to each, for tests and logs.
func (s *Service) Work(ctx context.Context) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for id, a := range s.actions {
		if a.State == "queued" || a.State == "executing" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return s.actions[ids[i]].Updated.Before(s.actions[ids[j]].Updated) })
	var out []string
	for _, id := range ids {
		out = append(out, s.execute(ctx, s.actions[id]))
	}
	return out
}

func (s *Service) guard(a *Action) string {
	now := s.now()
	if a.RollbackOf != "" {
		return "" // authorised compensating write: allowed after brief expiry and while frozen
	}
	if s.state.Frozen {
		return "frozen"
	}
	b, err := s.activeBrief(a.BriefID, a.BriefVersion)
	if err != nil {
		return err.(*Err).Code
	}
	_ = b
	if !now.Before(a.Expires) {
		return "action_expired"
	}
	if a.Approval == nil || a.Approval.PayloadHash != a.PayloadHash || a.Approval.TargetRevision != a.ExpectedRevision {
		return "approval_invalid"
	}
	if a.PolicyVersion != s.prof.PolicyVersion {
		return "policy_changed"
	}
	if asset := s.prof.Asset(a.TargetID); s.prof.Mode != "live" || asset == nil || !asset.WriteEnabled {
		return "writes_disabled"
	}
	return ""
}

// execute runs one change (caller holds s.mu). The page revision carries the action key,
// so a crash between writing the page and recording the action is settled on restart.
func (s *Service) execute(ctx context.Context, a *Action) string {
	page := s.pages[a.TargetID]
	if page == nil {
		return s.finish(ctx, a, "failed", "page_missing")
	}
	for _, r := range page.Revisions {
		if r.ActionKey == a.ActionKey { // receipt: the write already happened
			return s.applied(ctx, a, r.Revision, true)
		}
	}
	if a.State == "queued" {
		if reason := s.guard(a); reason == "frozen" {
			return "held:frozen"
		} else if reason != "" {
			return s.finish(ctx, a, "denied", reason)
		}
	}
	if page.Active != a.ExpectedRevision {
		s.audit(ctx, "worker", "action.conflict", "action:"+a.ID, map[string]any{"live_revision": page.Active})
		return s.finish(ctx, a, "conflict_review", "target_revision_conflict")
	}
	cur := page.current()
	if a.Previous == nil {
		a.Previous = map[string]string{}
		for f := range a.Patch {
			a.Previous[f] = cur.Fields[f]
		}
	}
	a.State, a.Updated = "executing", s.now().UTC()
	if err := s.put(ctx, kAction, a.ID, a); err != nil {
		return "error:" + err.Error()
	}
	fields := map[string]string{}
	for k, v := range cur.Fields {
		fields[k] = v
	}
	for k, v := range a.Patch {
		fields[k] = v
	}
	next := 0
	for _, r := range page.Revisions {
		next = max(next, r.Revision)
	}
	rev := PageRevision{Revision: next + 1, Fields: fields, ActionKey: a.ActionKey, At: s.now().UTC()}
	np := *page
	np.Revisions = append(append([]PageRevision(nil), page.Revisions...), rev)
	np.Active = rev.Revision
	if err := s.put(ctx, kPage, np.AssetID, &np); err != nil {
		return "error:" + err.Error() // the page is unchanged; the action stays executing and retries
	}
	s.pages[np.AssetID] = &np
	if CrashAfterWrite { // test hook: the process dies before recording the result
		return "crashed"
	}
	return s.applied(ctx, a, rev.Revision, false)
}

// CrashAfterWrite simulates a crash between the page write and the action record (tests).
var CrashAfterWrite = false

func (s *Service) applied(ctx context.Context, a *Action, rev int, reconciled bool) string {
	now := s.now().UTC()
	a.State, a.AppliedRevision, a.AppliedAt, a.LastError, a.Updated = "applied", rev, &now, "", now
	a.Receipt = map[string]any{"key": a.ActionKey, "revision": rev, "reconciled": reconciled}
	_ = s.put(ctx, kAction, a.ID, a)
	event := "action.applied"
	if a.RollbackOf != "" {
		event = "action.rolled_back"
		if orig := s.actions[a.RollbackOf]; orig != nil {
			orig.State, orig.Updated = "rolled_back", now
			_ = s.put(ctx, kAction, orig.ID, orig)
		}
	}
	s.audit(ctx, "worker", event, "action:"+a.ID, map[string]any{"revision": rev, "reconciled": reconciled})
	if reconciled {
		return "reconciled"
	}
	return "applied"
}

func (s *Service) finish(ctx context.Context, a *Action, st, reason string) string {
	a.State, a.LastError, a.Updated = st, reason, s.now().UTC()
	_ = s.put(ctx, kAction, a.ID, a)
	if st == "denied" {
		s.audit(ctx, "worker", "action.denied", "action:"+a.ID, map[string]any{"reason": reason})
	}
	if st == "conflict_review" {
		return "conflict"
	}
	return st + ":" + reason
}

// ---------------------------------------------------------------- pages and measurement

// CreatePage publishes the first revision of a guide page; an editor does this explicitly.
func (s *Service) CreatePage(ctx context.Context, assetID, actor string) (*Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	asset := s.prof.Asset(assetID)
	if asset == nil {
		return nil, notFound()
	}
	if s.pages[assetID] != nil {
		return nil, conflict("exists", "This page already exists.")
	}
	fields := map[string]string{}
	for _, f := range asset.AllowedFields {
		fields[f] = asset.Initial[f]
	}
	p := &Page{AssetID: assetID, Slug: asset.Slug, Active: 1, CreatedBy: actor,
		Revisions: []PageRevision{{Revision: 1, Fields: fields, At: s.now().UTC()}}}
	if err := s.put(ctx, kPage, assetID, p); err != nil {
		return nil, err
	}
	s.pages[assetID] = p
	s.audit(ctx, actor, "page.created", "page:"+assetID, map[string]any{"slug": asset.Slug})
	return p, nil
}

// Count records one anonymous event (a guide view or a join click). No cookie, no address,
// no identifier: a count per page revision and day.
func (s *Service) Count(ctx context.Context, assetID string, rev int, kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := fmt.Sprintf("%s|%d|%s|%s", assetID, rev, s.now().UTC().Format("2006-01-02"), kind)
	s.counts[k]++
	_ = s.put(ctx, kCount, k, s.counts[k])
}

const suppressBelow = 10

func shown(n int) any {
	if n < suppressBelow {
		return fmt.Sprintf("<%d", suppressBelow)
	}
	return n
}

// Report compares the windows before and after the first applied change (observational).
func (s *Service) Report(experimentID string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b *Brief
	for _, x := range s.briefs {
		if x.Experiment != nil && x.Experiment.ID == experimentID {
			b = x
		}
	}
	if b == nil {
		return nil, notFound()
	}
	e := b.Experiment
	out := map[string]any{"experiment_id": e.ID, "hypothesis": e.Hypothesis, "primary_outcome": e.Primary,
		"control_method": e.ControlMethod,
		"label":          "observational: page views before and after the change, no holdout; a view is not a person"}
	var applied *time.Time
	for _, a := range s.actions {
		if a.BriefID == b.str("brief_id") && a.BriefVersion == b.version() && a.Live && a.RollbackOf == "" && a.AppliedAt != nil {
			if applied == nil || a.AppliedAt.Before(*applied) {
				applied = a.AppliedAt
			}
		}
	}
	if applied == nil {
		out["status"], out["decision"] = "not_started", "insufficient_evidence"
		return out, nil
	}
	day := func(t time.Time) string { return t.UTC().Format("2006-01-02") }
	from, split, to := day(applied.Add(-time.Duration(e.BaselineDays)*24*time.Hour)), day(*applied), day(s.now())
	var v0, c0, v1, c1 int
	for k, n := range s.counts {
		parts := strings.Split(k, "|")
		if len(parts) != 4 || !slices.Contains(e.AssetIDs, parts[0]) {
			continue
		}
		d, kind := parts[2], parts[3]
		switch {
		case d >= from && d < split:
			if kind == "view" {
				v0 += n
			} else {
				c0 += n
			}
		case d >= split && d <= to:
			if kind == "view" {
				v1 += n
			} else {
				c1 += n
			}
		}
	}
	rate := func(c, v int) any {
		if v < suppressBelow {
			return nil
		}
		return math.Round(float64(c)/float64(v)*10000) / 10000
	}
	decision, reason := "insufficient_evidence", "Below the frozen minimum sample; keep observing."
	if v0 >= e.MinViews && v1 >= e.MinViews && c0+c1 >= e.MinOutcomes {
		r0, r1 := float64(c0)/float64(v0), float64(c1)/float64(v1)
		switch {
		case r1 < 0.8*r0:
			decision, reason = "rollback_recommended", "Guardrail: the join rate fell by more than 20 %."
		case r1 >= r0:
			decision, reason = "continue", "The rate held or rose (observational; not proven lift)."
		default:
			decision, reason = "revise", "The rate fell within the guardrail; consider revising."
		}
	}
	out["status"], out["applied_at"], out["decision"], out["reason"] = "running", applied.Format(time.RFC3339), decision, reason
	out["before"] = map[string]any{"views": shown(v0), "joins": shown(c0), "rate": rate(c0, v0)}
	out["after"] = map[string]any{"views": shown(v1), "joins": shown(c1), "rate": rate(c1, v1)}
	return out, nil
}

// SetFrozen is the emergency freeze: no new changes start; rollback still works.
func (s *Service) SetFrozen(ctx context.Context, frozen bool, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Frozen = frozen
	if err := s.saveState(ctx); err != nil {
		return err
	}
	ev := "growth.unfrozen"
	if frozen {
		ev = "growth.frozen"
	}
	s.audit(ctx, actor, ev, "growth", nil)
	return nil
}
