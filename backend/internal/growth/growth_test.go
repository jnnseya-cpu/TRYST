package growth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/staffauth"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
	"go.yaml.in/yaml/v3"
)

// Monday mid-day; the current complete week ends at 00:00 today.
var t0 = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
var weekEnd = time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)

func tokenHash(t string) string { h := sha256.Sum256([]byte(t)); return hex.EncodeToString(h[:]) }

type world struct {
	t     *testing.T
	st    store.Store
	now   time.Time
	svc   *Service
	api   http.Handler
	pub   http.Handler
	staff staffauth.Auth
}

func newWorld(t *testing.T, st store.Store) *world {
	t.Helper()
	if st == nil {
		st = store.NewMemory()
	}
	w := &world{t: t, st: st, now: t0}
	w.staff = staffauth.Auth{Env: "dev", DevTokens: map[string]staffauth.Staff{
		tokenHash("tok-growth"): {ID: "analyst@tryst", Roles: []string{"growth"}},
		tokenHash("tok-editor"): {ID: "editor@tryst", Roles: []string{"editor"}},
		tokenHash("tok-both"):   {ID: "solo@tryst", Roles: []string{"growth", "editor"}},
		tokenHash("tok-mod"):    {ID: "mod@tryst", Roles: []string{"moderator"}},
	}}
	w.reload()
	return w
}

// reload starts a fresh service on the same store (a restart).
func (w *world) reload() {
	svc, err := New(context.Background(), w.st, TrystProfile(), w.staff, func() time.Time { return w.now })
	if err != nil {
		w.t.Fatal(err)
	}
	w.svc = svc
	mux := http.NewServeMux()
	svc.Routes(mux)
	w.api = mux
	pub := http.NewServeMux()
	svc.PublicRoutes(pub, "https://www.tryst.test", "https://tryst.test")
	w.pub = pub
}

func (w *world) do(method, path, token string, body any, hdr ...string) (int, map[string]any) {
	w.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	r := httptest.NewRequest(method, path, rd)
	if token != "" {
		r.Header.Set("Authorization", "Staff "+token)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	w.api.ServeHTTP(rec, r)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func weekly(source, term string, current float64, hist []float64, hash string) []Observation {
	vals := append(append([]float64(nil), hist...), current)
	var out []Observation
	for i, v := range vals {
		we := weekEnd.AddDate(0, 0, -7*(len(vals)-1-i))
		o := Observation{SourceID: source, SourceRecordID: fmt.Sprintf("%s-%s-%s", source, strings.ReplaceAll(term, " ", "_"), we.Format("20060102")),
			Term: term, Market: "GB", Locale: "en-GB", Metric: "count", Value: v, Unit: "count", Coverage: 1,
			WindowStart: we.AddDate(0, 0, -7), WindowEnd: we, ObservedAt: we}
		if hash != "" {
			o.EvidenceHash = hash + we.Format("20060102")
		}
		out = append(out, o)
	}
	return out
}

var hist = []float64{120, 131, 118, 126, 124, 129, 121, 127}

// platformLondon is what the member platform sends: weekly distinct-member counts.
func platformLondon() []Observation {
	return weekly("platform_tonight", "tonight start London", 310, hist, "")
}

// searchLondon is a staff import from Search Console.
func searchLondon() []Observation {
	return weekly("search_console", "discreet dating london", 220, []float64{60, 64, 58, 61, 63, 59, 62, 60}, "")
}

type fakePlatform []Observation

func (f fakePlatform) Aggregates(context.Context) ([]Observation, error) { return f, nil }

// ready ingests London signals, scores, and returns the eligible London snapshot id.
func (w *world) ready() string {
	w.t.Helper()
	w.svc.Signals = fakePlatform(platformLondon())
	code, out := w.do("POST", "/ops/v1/growth/observations", "tok-growth", map[string]any{"items": searchLondon()}, "Idempotency-Key", "k1")
	if code != 200 {
		w.t.Fatalf("ingest %d %v", code, out)
	}
	if code, out := w.do("POST", "/ops/v1/growth/refresh", "tok-growth", nil); code != 202 {
		w.t.Fatalf("refresh %d %v", code, out)
	}
	for _, s := range w.svc.Opportunities("") {
		if s.TopicID == "discreet-dating-london" {
			if s.Status != "eligible" {
				w.t.Fatalf("london not eligible: %+v %v %v", s.Components, s.ConfidenceIn, s.Reasons)
			}
			return s.ID
		}
	}
	w.t.Fatal("no london opportunity")
	return ""
}

func (w *world) approvedBrief() map[string]any {
	w.t.Helper()
	snap := w.ready()
	code, b := w.do("POST", "/ops/v1/growth/briefs", "tok-growth", map[string]any{"snapshot_id": snap})
	if code != 201 {
		w.t.Fatalf("brief %d %v", code, b)
	}
	code, b = w.do("POST", "/ops/v1/growth/briefs/"+b["id"].(string)+"/approve", "tok-editor", nil, "If-Match", "1")
	if code != 200 {
		w.t.Fatalf("approve %d %v", code, b)
	}
	if code, out := w.do("POST", "/ops/v1/growth/pages", "tok-editor", map[string]any{"asset_id": "guide_london"}); code != 201 {
		w.t.Fatalf("page %d %v", code, out)
	}
	return b
}

func (w *world) drafts(briefID string) []map[string]any {
	w.t.Helper()
	code, out := w.do("POST", "/ops/v1/growth/briefs/"+briefID+"/drafts", "tok-growth", nil)
	if code != 201 {
		w.t.Fatalf("drafts %d %v", code, out)
	}
	var acts []map[string]any
	for _, a := range out["actions"].([]any) {
		acts = append(acts, a.(map[string]any))
	}
	return acts
}

func (w *world) approveAndQueue(a map[string]any) {
	w.t.Helper()
	code, out := w.do("POST", "/ops/v1/growth/actions/"+a["id"].(string)+"/approve", "tok-editor",
		map[string]any{"payload_hash": a["payload_hash"], "target_revision": a["expected_target_revision"]})
	if code != 200 {
		w.t.Fatalf("approve action %d %v", code, out)
	}
	if code, out := w.do("POST", "/ops/v1/growth/actions/"+a["id"].(string)+"/execute", "tok-editor", nil); code != 202 {
		w.t.Fatalf("execute %d %v", code, out)
	}
}

func (w *world) page(slug string) (int, string, http.Header) {
	r := httptest.NewRequest("GET", "/guides/"+slug, nil)
	rec := httptest.NewRecorder()
	w.pub.ServeHTTP(rec, r)
	return rec.Code, rec.Body.String(), rec.Header()
}

func kind(acts []map[string]any, k string) map[string]any {
	for _, a := range acts {
		if a["action_kind"] == k {
			return a
		}
	}
	return nil
}

// ---------------------------------------------------------------------------------------

func TestReferenceScoresAreUnrounded(t *testing.T) { // TI-03
	s, c := OpportunityScore(80, 95, 85, 90, 80), Confidence(90, 80, 70, 90, 2)
	if s != 86.25 || c != 83.5 {
		t.Fatalf("got %v %v", s, c)
	}
	if st, _ := Decide(s, c, nil, false, false); st != "eligible" {
		t.Fatal(st)
	}
	if st, _ := Decide(74.999, 90, nil, false, false); st != "watching" {
		t.Fatal(st)
	}
	if Confidence(100, 100, 100, 100, 1) != 69 {
		t.Fatal("single family must cap at 69")
	}
	m := CountMovement(310, hist, 50, 2)
	if !m.PassesGate || m.DemandScore != clamp(50+10**m.Z) {
		t.Fatalf("%+v", m)
	}
	if CountMovement(19, []float64{1, 1, 1, 1}, 20, 2).PassesGate || !CountMovement(500, []float64{1, 2}, 20, 2).InsufficientHistory {
		t.Fatal("gates")
	}
	if !IndexMovement(12, []float64{3, 4, 3, 4, 3}, 2).PassesGate {
		t.Fatal("index feeds have no raw-count gate")
	}
}

func TestTrystProfileClaimsAreTheAuditedWording(t *testing.T) {
	p := TrystProfile()
	for _, c := range p.Claims {
		file := strings.SplitN(c.Source, " ", 2)[0]
		b, err := os.ReadFile(filepath.Join("..", "..", "..", file))
		if err != nil {
			t.Fatal(err)
		}
		norm := func(s string) string { return strings.ReplaceAll(strings.ToLower(s), "’", "'") }
		core := strings.TrimSuffix(c.Text, ".")
		if c.ID == "claim_tonight" {
			core = "only visible to people who are also free tonight, and only for the hours you choose"
		}
		if !strings.Contains(norm(string(b)), norm(core)) {
			t.Errorf("%s is not the wording in %s", c.ID, file)
		}
	}
	for _, w := range []string{"Safe", "vetted", "guaranteed", "#1", "AI"} {
		if p.Prohibited("Totally "+w+" dating") == "" {
			t.Errorf("%q should be prohibited", w)
		}
	}
	if p.Prohibited("said and done") != "" || p.Prohibited("Every member is age assured") != "" {
		t.Error("prohibited words must match whole words only")
	}
	if _, err := LoadProfile([]byte(`{"schema_version":1,"api_key":"x"}`)); err == nil {
		t.Error("unknown profile fields (such as a key) must be refused")
	}
}

func TestBriefContractMatchesTheSchemaAndFixtures(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "shared", "contracts", "growth")
	raw, err := os.ReadFile(filepath.Join(dir, "growth-brief.v1.schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Required   []string       `yaml:"required"`
		Properties map[string]any `yaml:"properties"`
	}
	if err := yaml.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(schema.Required, BriefFields) || len(schema.Properties) != len(BriefFields) {
		t.Fatalf("BriefFields drifted from the schema: %v", schema.Required)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "fixtures", "*.json"))
	if len(files) < 10 {
		t.Fatal("fixtures missing")
	}
	for _, f := range files {
		b, _ := os.ReadFile(f)
		var body map[string]any
		if err := json.Unmarshal(b, &body); err != nil {
			t.Fatal(err)
		}
		errs := BriefSchemaErrors(body)
		pos := strings.Contains(filepath.Base(f), "positive")
		if pos && len(errs) > 0 || !pos && len(errs) == 0 {
			t.Errorf("%s: %v", filepath.Base(f), errs)
		}
	}
}

func TestBriefToPublishedPageAndBack(t *testing.T) {
	w := newWorld(t, nil)
	b := w.approvedBrief()
	body := b["body"].(map[string]any)
	if errs := BriefSchemaErrors(body); len(errs) > 0 {
		t.Fatal(errs)
	}
	code, eff := w.do("GET", "/ops/v1/growth/briefs/effective?market=GB&locale=en-GB&topic=discreet-dating-london&channel=seo", "tok-growth", nil)
	if code != 200 || eff["brief_id"] != body["brief_id"] {
		t.Fatalf("effective %d %v", code, eff)
	}
	acts := w.drafts(b["id"].(string))
	seo, mkt, social := kind(acts, "seo_metadata_patch"), kind(acts, "owned_content_patch"), kind(acts, "organic_social_draft")
	if seo == nil || mkt == nil || social == nil || social["state"] != "validated" {
		t.Fatalf("drafts %v", acts)
	}
	w.approveAndQueue(seo)
	if got := w.svc.Work(context.Background()); !slices.Equal(got, []string{"applied"}) {
		t.Fatal(got)
	}
	// The second change was drafted against revision 1; the page is now at 2.
	code, out := w.do("POST", "/ops/v1/growth/actions", "tok-growth", proposal(mkt, 1), "Idempotency-Key", "m1")
	if code != 409 || out["code"] != "target_revision_conflict" {
		t.Fatalf("stale %d %v", code, out)
	}
	code, rebased := w.do("POST", "/ops/v1/growth/actions", "tok-growth", proposal(mkt, 2), "Idempotency-Key", "m2")
	if code != 201 {
		t.Fatalf("rebase %d %v", code, rebased)
	}
	w.approveAndQueue(rebased)
	if got := w.svc.Work(context.Background()); !slices.Equal(got, []string{"applied"}) {
		t.Fatal(got)
	}
	code, html, hdr := w.page("discreet-dating-london")
	if code != 200 || !strings.Contains(html, "Discreet dating London on TRYST") || !strings.Contains(html, "With Tonight") {
		t.Fatalf("%d %s", code, html)
	}
	if hdr.Get("Set-Cookie") != "" || strings.Contains(strings.ToLower(html), "<script") {
		t.Fatal("guide pages set no cookie and run no script")
	}
	for _, word := range []string{"growth", "brief", "generated", " ai ", "automat", "algorithm"} {
		if strings.Contains(strings.ToLower(html), word) {
			t.Errorf("guide page reveals the machinery: %q", word)
		}
	}
	// Views and join clicks, counted per revision and day; then the report.
	w.now = t0.Add(3 * 24 * time.Hour)
	for range 30 {
		w.page("discreet-dating-london")
	}
	rec := httptest.NewRecorder()
	w.pub.ServeHTTP(rec, httptest.NewRequest("POST", "/guides/discreet-dating-london/join", nil))
	if rec.Code != 303 || rec.Header().Get("Location") != "https://tryst.test/join" {
		t.Fatalf("join %d %s", rec.Code, rec.Header().Get("Location"))
	}
	code, rep := w.do("GET", "/ops/v1/growth/experiments/"+body["experiment_id"].(string), "tok-growth", nil)
	if code != 200 || rep["decision"] != "insufficient_evidence" || !strings.HasPrefix(rep["label"].(string), "observational") {
		t.Fatalf("report %d %v", code, rep)
	}
	if rep["after"].(map[string]any)["joins"] != "<10" {
		t.Fatalf("small counts must be suppressed: %v", rep["after"])
	}
	// Roll back the latest change; the earlier one can't be restored over it (§6).
	if code, out := w.do("POST", "/ops/v1/growth/actions/"+rebased["id"].(string)+"/rollback", "tok-editor", nil); code != 202 {
		t.Fatalf("rollback %d %v", code, out)
	}
	if got := w.svc.Work(context.Background()); !slices.Equal(got, []string{"applied"}) {
		t.Fatal(got)
	}
	if _, html, _ := w.page("discreet-dating-london"); !strings.Contains(html, "<h1>Meeting discreetly in London</h1>") ||
		!strings.Contains(html, "<title>Discreet dating London on TRYST</title>") { // marketing undone, SEO kept
		t.Fatalf("not restored: %s", html)
	}
	w.do("POST", "/ops/v1/growth/actions/"+seo["id"].(string)+"/rollback", "tok-editor", nil)
	if got := w.svc.Work(context.Background()); !slices.Equal(got, []string{"conflict"}) {
		t.Fatal(got)
	}
	_, audit := w.do("GET", "/ops/v1/growth/audit", "tok-growth", nil)
	seen := map[string]bool{}
	for _, e := range audit["items"].([]any) {
		seen[e.(map[string]any)["event"].(string)] = true
	}
	for _, ev := range []string{"observations.ingested", "scoring.run", "brief.drafted", "brief.approved", "page.created",
		"action.proposed", "action.approved", "action.queued", "action.applied", "action.rollback_queued", "action.rolled_back", "action.conflict"} {
		if !seen[ev] {
			t.Errorf("audit lacks %s", ev)
		}
	}
}

func proposal(a map[string]any, rev int) map[string]any {
	return map[string]any{"brief_id": a["brief_id"], "brief_version": a["brief_version"], "action_kind": a["action_kind"],
		"channel": a["channel"], "target_id": a["target_id"], "expected_target_revision": rev, "patch": a["patch"],
		"evidence_ids": a["evidence_ids"], "approved_claim_ids": a["approved_claim_ids"]}
}

func TestIngestReplayAndConflict(t *testing.T) { // TI-01
	w := newWorld(t, nil)
	items := searchLondon()[:3]
	code, first := w.do("POST", "/ops/v1/growth/observations", "tok-growth", map[string]any{"items": items}, "Idempotency-Key", "a")
	if code != 200 || first["accepted"] != 3.0 {
		t.Fatal(code, first)
	}
	_, again := w.do("POST", "/ops/v1/growth/observations", "tok-growth", map[string]any{"items": items}, "Idempotency-Key", "b")
	if again["duplicate"] != 3.0 {
		t.Fatal(again)
	}
	items[1].Value++
	if code, out := w.do("POST", "/ops/v1/growth/observations", "tok-growth", map[string]any{"items": items}, "Idempotency-Key", "c"); code != 409 || out["code"] != "observation_conflict" {
		t.Fatal(code, out)
	}
	if code, out := w.do("POST", "/ops/v1/growth/observations", "tok-growth", map[string]any{"items": items}, "Idempotency-Key", "a"); code != 409 || out["code"] != "idempotency_conflict" {
		t.Fatal(code, out)
	}
	if code, _ := w.do("POST", "/ops/v1/growth/observations", "tok-growth", map[string]any{"items": items}); code != 428 {
		t.Fatal("mutations need an Idempotency-Key")
	}
	forged := platformLondon()[:1]
	if code, out := w.do("POST", "/ops/v1/growth/observations", "tok-growth", map[string]any{"items": forged}, "Idempotency-Key", "f"); code != 422 {
		t.Fatal("staff must not be able to type in the member platform's counts", code, out)
	}
	pii := []Observation{searchLondon()[0]}
	pii[0].Term, pii[0].SourceRecordID = "write to someone@example.com", "x1"
	if code, out := w.do("POST", "/ops/v1/growth/observations", "tok-growth", map[string]any{"items": pii}, "Idempotency-Key", "d"); code != 422 {
		t.Fatal("personal data must be refused", code, out)
	}
}

func TestSyndicatedCopiesCountOnceAndStaleCannotActivate(t *testing.T) { // TI-04
	w := newWorld(t, nil)
	// The search export mirrors the platform's own count: same provenance, one family.
	obs := append(weekly("platform_tonight", "tonight start London", 310, hist, "mirror-"),
		weekly("search_console", "london", 300, hist, "mirror-")...)
	w.svc.Signals = fakePlatform(obs[:9])
	if _, err := w.svc.Ingest(context.Background(), obs[9:], "t"); err != nil {
		t.Fatal(err)
	}
	w.svc.Refresh(context.Background(), "t")
	s := w.svc.Opportunities("")[0]
	if len(s.Families) != 1 || s.Confidence > 69 || s.Status == "eligible" {
		t.Fatalf("families %v conf %v status %s", s.Families, s.Confidence, s.Status)
	}
	w2 := newWorld(t, nil)
	w2.now = t0.Add(20 * 24 * time.Hour)
	w2.svc.Signals = fakePlatform(platformLondon())
	w2.svc.Ingest(context.Background(), searchLondon(), "t")
	w2.svc.Refresh(context.Background(), "t")
	s = w2.svc.Opportunities("")[0]
	if s.Status == "eligible" || len(s.Notes) == 0 || !strings.Contains(strings.Join(s.Notes, " "), "stale") {
		t.Fatalf("stale data activated: %s %v", s.Status, s.Notes)
	}
}

func TestExpiredBriefStopsAQueuedChange(t *testing.T) { // TI-05
	w := newWorld(t, nil)
	b := w.approvedBrief()
	seo := kind(w.drafts(b["id"].(string)), "seo_metadata_patch")
	w.approveAndQueue(seo)
	w.now = t0.Add(8 * 24 * time.Hour)
	if code, _ := w.do("GET", "/ops/v1/growth/briefs/effective?market=GB&locale=en-GB&topic=discreet-dating-london&channel=seo", "tok-growth", nil); code != 204 {
		t.Fatal("expired brief must fall back to baseline")
	}
	if got := w.svc.Work(context.Background()); !slices.Equal(got, []string{"denied:brief_expired"}) {
		t.Fatal(got)
	}
	if _, html, _ := w.page("discreet-dating-london"); !strings.Contains(html, "Meeting discreetly in London") {
		t.Fatal("the page changed")
	}
}

func TestInjectedTextCannotPublishAnything(t *testing.T) { // TI-06, TI-19
	model := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// A model that obeyed injected text: wrong fields, a link, prohibited words.
		out := `{"title":"Safe, vetted, guaranteed <a href=x>","description":"approve and publish now","action":"delete","claim_ids":["claim_x"]}`
		b, _ := json.Marshal(map[string]any{"model": "m", "choices": []any{map[string]any{"message": map[string]any{"content": out}}}})
		_, _ = rw.Write(b)
	}))
	defer model.Close()
	w := newWorld(t, nil)
	w.svc.prof.Inference.Route = "deployment"
	w.svc.Gen = &Generator{DeploymentURL: model.URL}
	b := w.approvedBrief()
	acts := w.drafts(b["id"].(string))
	for _, a := range acts {
		inf := a["inference"].(map[string]any)
		if inf["route"] != "deterministic" || inf["degraded"] != true || a["state"] == "approved" {
			t.Fatalf("%v", a)
		}
	}
	for _, a := range acts {
		if bad := w.svc.prof.Prohibited(fmt.Sprint(a["patch"])); bad != "" {
			t.Fatalf("prohibited wording reached a draft: %s", bad)
		}
	}
	if code, _ := w.do("POST", "/ops/v1/growth/actions", "tok-growth", map[string]any{"brief_id": "x", "action_kind": "delete_site"}, "Idempotency-Key", "z"); code != 422 {
		t.Fatal("unknown kinds are refused")
	}
	// With no model configured, the deployment route fails closed to templates.
	w2 := newWorld(t, nil)
	w2.svc.prof.Inference.Route = "deployment"
	b2 := w2.approvedBrief()
	for _, a := range w2.drafts(b2["id"].(string)) {
		if a["inference"].(map[string]any)["route"] != "deterministic" {
			t.Fatal(a)
		}
	}
}

func TestCrashAfterPageWriteIsSettledOnRestart(t *testing.T) { // TI-07
	st := store.NewMemory()
	w := newWorld(t, st)
	b := w.approvedBrief()
	seo := kind(w.drafts(b["id"].(string)), "seo_metadata_patch")
	w.approveAndQueue(seo)
	CrashAfterWrite = true
	got := w.svc.Work(context.Background())
	CrashAfterWrite = false
	if !slices.Equal(got, []string{"crashed"}) {
		t.Fatal(got)
	}
	w.reload() // the process restarts from the database
	if got := w.svc.Work(context.Background()); !slices.Equal(got, []string{"reconciled"}) {
		t.Fatal(got)
	}
	p := w.svc.pages["guide_london"]
	if p.Active != 2 || len(p.Revisions) != 2 {
		t.Fatalf("one write only: %+v", p)
	}
}

func TestApprovalRulesRolesAndFreeze(t *testing.T) {
	w := newWorld(t, nil)
	snap := w.ready()
	_, b := w.do("POST", "/ops/v1/growth/briefs", "tok-both", map[string]any{"snapshot_id": snap})
	if code, out := w.do("POST", "/ops/v1/growth/briefs/"+b["id"].(string)+"/approve", "tok-both", nil, "If-Match", "1"); code != 403 || out["code"] != "self_approval" {
		t.Fatal(code, out)
	}
	if code, _ := w.do("POST", "/ops/v1/growth/briefs/"+b["id"].(string)+"/approve", "tok-editor", nil); code != 428 {
		t.Fatal("If-Match is required")
	}
	if code, _ := w.do("POST", "/ops/v1/growth/briefs/"+b["id"].(string)+"/approve", "tok-growth", nil, "If-Match", "1"); code != 403 {
		t.Fatal("approving needs the editor role")
	}
	if code, _ := w.do("GET", "/ops/v1/growth/overview", "", nil); code != 401 {
		t.Fatal("signed out")
	}
	if code, _ := w.do("GET", "/ops/v1/growth/overview", "tok-mod", nil); code != 401 {
		t.Fatal("moderators have no growth access (least privilege)")
	}
	w.do("POST", "/ops/v1/growth/briefs/"+b["id"].(string)+"/approve", "tok-editor", nil, "If-Match", "1")
	w.do("POST", "/ops/v1/growth/pages", "tok-editor", map[string]any{"asset_id": "guide_london"})
	seo := kind(w.drafts(b["id"].(string)), "seo_metadata_patch")
	if code, out := w.do("POST", "/ops/v1/growth/actions/"+seo["id"].(string)+"/approve", "tok-editor",
		map[string]any{"payload_hash": strings.Repeat("0", 64), "target_revision": 1}); code != 409 || out["code"] != "payload_mismatch" {
		t.Fatal(code, out)
	}
	w.approveAndQueue(seo)
	w.do("POST", "/ops/v1/growth/freeze", "tok-editor", map[string]any{"frozen": true})
	if got := w.svc.Work(context.Background()); !slices.Equal(got, []string{"held:frozen"}) {
		t.Fatal(got)
	}
	w.do("POST", "/ops/v1/growth/freeze", "tok-editor", map[string]any{"frozen": false})
	w.svc.Work(context.Background())
	w.do("POST", "/ops/v1/growth/freeze", "tok-editor", map[string]any{"frozen": true})
	if code, _ := w.do("POST", "/ops/v1/growth/actions/"+seo["id"].(string)+"/rollback", "tok-editor", nil); code != 202 {
		t.Fatal("rollback works while frozen")
	}
	if got := w.svc.Work(context.Background()); !slices.Equal(got, []string{"applied"}) {
		t.Fatal(got)
	}
}

func TestGuidePagesAreInvisibleUntilAnEditorCreatesThem(t *testing.T) {
	w := newWorld(t, nil)
	if code, _, _ := w.page("discreet-dating-london"); code != 404 {
		t.Fatal("no page exists before an editor creates it")
	}
	rec := httptest.NewRecorder()
	w.pub.ServeHTTP(rec, httptest.NewRequest("GET", "/ops/v1/growth/overview", nil))
	if rec.Code != 404 {
		t.Fatal("the public listener serves guide pages only")
	}
	w.do("POST", "/ops/v1/growth/pages", "tok-editor", map[string]any{"asset_id": "guide_london"})
	code, html, hdr := w.page("discreet-dating-london")
	if code != 200 || !strings.Contains(hdr.Get("Content-Security-Policy"), "default-src 'none'") ||
		!strings.Contains(html, `rel="canonical" href="https://www.tryst.test/guides/discreet-dating-london"`) {
		t.Fatal(code, hdr, html)
	}
}
