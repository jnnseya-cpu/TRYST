package safetysvc

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

// --- fakes ---

type members struct{}

func (members) Authenticate(r *http.Request) (identity.Account, bool) {
	id := r.Header.Get("X-Test-Member")
	return identity.Account{ID: id}, id != ""
}

type directory struct {
	mu       sync.Mutex
	threads  map[string][2]string
	standing map[string]string
}

func newDirectory() *directory {
	return &directory{threads: map[string][2]string{
		"th_ab": {"alice", "bob"}, "th_cb": {"carol", "bob"}, "th_db": {"dave", "bob"},
	}, standing: map[string]string{}}
}

func (d *directory) Counterpart(me, id string) (string, bool) {
	t, ok := d.threads[id]
	switch {
	case !ok:
		return "", false
	case t[0] == me:
		return t[1], true
	case t[1] == me:
		return t[0], true
	}
	return "", false
}
func (d *directory) Summary(a string) (string, string) {
	return strings.ToUpper(a[:1]) + a[1:], "London"
}
func (d *directory) MainPhoto(_ context.Context, a string) ([]byte, string, error) {
	if a == "bob" {
		return []byte("jpeg-bytes"), "image/jpeg", nil
	}
	return nil, "", errors.New("none")
}
func (d *directory) SetStanding(a, s string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.standing[a] = s
}
func (d *directory) get(a string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.standing[a]
}

// prefixed scopes a shared test database to one run.
type prefixed struct {
	store.Store
	ns string
}

func (p prefixed) Put(ctx context.Context, k, id string, b []byte) error {
	return p.Store.Put(ctx, p.ns+k, id, b)
}
func (p prefixed) All(ctx context.Context, k string) (map[string][]byte, error) {
	return p.Store.All(ctx, p.ns+k)
}

const (
	tokMod1 = "mod-one-token-0123456789"
	tokMod2 = "mod-two-token-0123456789"
	tokLead = "lead-token-0123456789abc"
)

func hashTok(t string) string { h := sha256.Sum256([]byte(t)); return hex.EncodeToString(h[:]) }

func devStaff() StaffAuth {
	return StaffAuth{Env: "dev", DevTokens: map[string]Staff{
		hashTok(tokMod1): {ID: "mod1", Roles: []string{"moderator"}},
		hashTok(tokMod2): {ID: "mod2", Roles: []string{"moderator"}},
		hashTok(tokLead): {ID: "lead1", Roles: []string{"lead"}},
	}}
}

type rig struct {
	t   *testing.T
	svc *Service
	dir *directory
}

func newRig(t *testing.T, st store.Store) *rig {
	t.Helper()
	dir := newDirectory()
	svc, err := New(members{}, devStaff(), dir, st)
	if err != nil {
		t.Fatal(err)
	}
	return &rig{t, svc, dir}
}

func (g *rig) do(method, path, member, staffTok string, body any) (*httptest.ResponseRecorder, map[string]any) {
	g.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	r := httptest.NewRequest(method, path, rd)
	if member != "" {
		r.Header.Set("X-Test-Member", member)
	}
	if staffTok != "" {
		r.Header.Set("Authorization", "Staff "+staffTok)
	}
	w := httptest.NewRecorder()
	g.svc.ServeHTTP(w, r)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func (g *rig) report(member, thread, cat string) string {
	g.t.Helper()
	w, out := g.do("POST", "/v1/reports", member, "", map[string]any{"thread_id": thread, "category": cat, "disclosed": []string{"you'd better pay"}})
	if w.Code != 201 {
		g.t.Fatalf("report: %d %s", w.Code, w.Body)
	}
	return out["reference"].(string)
}

// --- tests ---

func TestReportOpensCaseWithDeadlineAndHidesReporter(t *testing.T) {
	g := newRig(t, store.NewMemory())
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	g.svc.now = func() time.Time { return now }

	w, out := g.do("GET", "/v1/reports/categories", "alice", "", nil)
	if w.Code != 200 || len(out["categories"].([]any)) != len(categories) {
		t.Fatalf("categories: %d %s", w.Code, w.Body)
	}
	if w, _ := g.do("POST", "/v1/reports", "alice", "", map[string]any{"thread_id": "th_cb", "category": "threat_violence"}); w.Code != 404 {
		t.Fatalf("report on someone else's thread: %d", w.Code)
	}
	if w, _ := g.do("POST", "/v1/reports", "", "", map[string]any{"thread_id": "th_ab", "category": "threat_violence"}); w.Code != 401 {
		t.Fatalf("unauthenticated report: %d", w.Code)
	}
	if w, _ := g.do("POST", "/v1/reports", "alice", "", map[string]any{"thread_id": "th_ab", "category": "nonsense"}); w.Code != 422 {
		t.Fatalf("bad category: %d", w.Code)
	}
	id := g.report("alice", "th_ab", "sextortion")
	w, out = g.do("GET", "/ops/v1/cases/"+id, "", tokMod1, nil)
	if w.Code != 200 {
		t.Fatalf("case: %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "alice") || out["reporter"] != "hidden" {
		t.Fatalf("reporter leaked to the console: %s", w.Body)
	}
	cs := out["case"].(map[string]any)
	if cs["due"] != now.Add(time.Hour).Format(time.RFC3339) {
		t.Fatalf("sextortion deadline = %v, want 1h", cs["due"])
	}
	if out["subject"].(map[string]any)["name"] != "Bob" {
		t.Fatalf("subject: %v", out["subject"])
	}
	// Minor reports are due in 15 minutes and go to the front of the queue.
	g.report("carol", "th_cb", "minor")
	_, q := g.do("GET", "/ops/v1/cases", "", tokMod1, nil)
	list := q["cases"].([]any)
	if len(list) != 2 || list[0].(map[string]any)["category"] != "minor" {
		t.Fatalf("queue order: %v", list)
	}
}

func TestStaffAuth(t *testing.T) {
	g := newRig(t, store.NewMemory())
	if w, _ := g.do("GET", "/ops/v1/cases", "", "", nil); w.Code != 401 {
		t.Fatalf("no staff: %d", w.Code)
	}
	if w, _ := g.do("GET", "/ops/v1/cases", "alice", "", nil); w.Code != 401 {
		t.Fatalf("member on console: %d", w.Code)
	}
	if w, _ := g.do("GET", "/ops/v1/cases", "", "wrong-token-0123456789", nil); w.Code != 401 {
		t.Fatalf("wrong token: %d", w.Code)
	}
	if _, err := New(members{}, StaffAuth{Env: "prod", DevTokens: devStaff().DevTokens}, newDirectory(), store.NewMemory()); !errors.Is(err, ErrDevTokensInProd) {
		t.Fatalf("dev tokens in prod: %v", err)
	}
	// Production: identity headers are believed only from the staff sign-in proxy.
	a := StaffAuth{Env: "prod", TrustedProxies: []string{"10.0.0.5"}}
	r := httptest.NewRequest("GET", "/ops/v1/cases", nil)
	r.Header.Set("X-Staff-Identity", "mallory")
	r.Header.Set("X-Staff-Roles", "lead")
	r.RemoteAddr = "203.0.113.9:4000"
	if _, ok := a.Authenticate(r); ok {
		t.Fatal("staff header believed from an untrusted address")
	}
	r.RemoteAddr = "10.0.0.5:4000"
	if s, ok := a.Authenticate(r); !ok || s.ID != "mallory" || !s.has("lead") {
		t.Fatal("staff header from the trusted proxy was refused")
	}
	r.Header.Set("Authorization", "Staff "+tokMod1)
	r.Header.Del("X-Staff-Identity")
	if _, ok := (StaffAuth{Env: "prod", DevTokens: devStaff().DevTokens}).Authenticate(r); ok {
		t.Fatal("dev token accepted in prod")
	}
}

func TestPhotoRevealNeedsReasonAndIsAudited(t *testing.T) {
	g := newRig(t, store.NewMemory())
	id := g.report("alice", "th_ab", "impersonation")
	if w, _ := g.do("GET", "/ops/v1/cases/"+id+"/photo", "", tokMod1, nil); w.Code != 422 {
		t.Fatalf("photo without reason: %d", w.Code)
	}
	w, _ := g.do("GET", "/ops/v1/cases/"+id+"/photo?reason=compare+with+the+reported+catfish+photo", "", tokMod1, nil)
	if w.Code != 200 || w.Body.String() != "jpeg-bytes" {
		t.Fatalf("photo: %d %s", w.Code, w.Body)
	}
	last := g.svc.chain[len(g.svc.chain)-1]
	if last.Action != "reveal_photo" || last.Staff != "mod1" || !strings.Contains(last.Detail, "catfish") {
		t.Fatalf("audit: %+v", last)
	}
}

func TestSuspectedMinorFreezesUntilDecided(t *testing.T) {
	g := newRig(t, store.NewMemory())
	id := g.report("alice", "th_ab", "minor")
	if g.dir.get("bob") != "suspend" {
		t.Fatalf("suspected minor not frozen: %q", g.dir.get("bob"))
	}
	_, st := g.do("GET", "/v1/me/standing", "bob", "", nil)
	if st["standing"] != "suspend" || st["under_review"] != true {
		t.Fatalf("member view of the freeze: %v", st)
	}
	// One uncorroborated report can't remove anyone permanently.
	w, _ := g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "uphold", "reason": "Reported as under 18"})
	if w.Code != 422 {
		t.Fatalf("uncorroborated permanent: %d %s", w.Code, w.Body)
	}
	if w, _ := g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "dismiss", "reason": "Verified adult; photo matches ID check"}); w.Code != 200 {
		t.Fatalf("dismiss: %d", w.Code)
	}
	if g.dir.get("bob") != "good" {
		t.Fatalf("freeze not lifted on dismissal: %q", g.dir.get("bob"))
	}
}

func TestCorroboratingReportsCloseTogether(t *testing.T) {
	g := newRig(t, store.NewMemory())
	g.report("alice", "th_ab", "harassment")
	id := g.report("carol", "th_cb", "harassment")
	g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "uphold", "reason": "Two members describe pressure after a no"})
	_, q := g.do("GET", "/ops/v1/cases", "", tokMod1, nil)
	if n := len(q["cases"].([]any)); n != 0 {
		t.Fatalf("corroborating case left open: %d", n)
	}
	_, st := g.do("GET", "/v1/me/standing", "bob", "", nil)
	if len(st["decisions"].([]any)) != 1 || st["standing"] != "warn" {
		t.Fatalf("one decision expected: %v", st)
	}
}

func TestLadder(t *testing.T) {
	g := newRig(t, store.NewMemory())
	// Harassment: warn, then shadow limit, then suspend.
	want := []string{"warn", "shadow_limit", "suspend"}
	for i, reporter := range []string{"alice", "carol", "dave"} {
		th := map[string]string{"alice": "th_ab", "carol": "th_cb", "dave": "th_db"}[reporter]
		cid := g.report(reporter, th, "harassment")
		w, out := g.do("POST", "/ops/v1/cases/"+cid+"/decision", "", tokMod1, map[string]any{"outcome": "uphold", "reason": "Repeated messages after a clear no"})
		if w.Code != 200 {
			t.Fatalf("decision %d: %d %s", i, w.Code, w.Body)
		}
		if got := out["decision"].(map[string]any)["action"]; got != want[i] {
			t.Fatalf("strike %d = %v, want %s", i, got, want[i])
		}
	}
	if g.dir.get("bob") != "suspend" {
		t.Fatalf("standing = %q", g.dir.get("bob"))
	}
	w, out := g.do("GET", "/v1/me/standing", "bob", "", nil)
	if w.Code != 200 || out["standing"] != "suspend" || len(out["decisions"].([]any)) != 3 {
		t.Fatalf("standing: %s", w.Body)
	}
	// Dismissing doesn't change standing; a decided case can't be decided again.
	id := g.report("alice", "th_ab", "other")
	if w, _ := g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "dismiss", "reason": "Nothing against the rules"}); w.Code != 200 {
		t.Fatalf("dismiss: %d", w.Code)
	}
	if g.dir.get("bob") != "suspend" {
		t.Fatalf("dismissal changed standing: %q", g.dir.get("bob"))
	}
	if w, _ := g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "dismiss", "reason": "Nothing against the rules"}); w.Code != 409 {
		t.Fatalf("second decision: %d", w.Code)
	}
}

func TestPermanentRemovalNeedsSecondModerator(t *testing.T) {
	g := newRig(t, store.NewMemory())
	// Two independent reporters corroborate.
	g.report("alice", "th_ab", "sextortion")
	id := g.report("carol", "th_cb", "sextortion")
	w, out := g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "uphold", "reason": "Two members describe the same blackmail"})
	if w.Code != 200 || out["state"] != "pending_second" {
		t.Fatalf("propose: %d %s", w.Code, w.Body)
	}
	if g.dir.get("bob") != "suspend" {
		t.Fatalf("interim standing = %q, want suspend", g.dir.get("bob"))
	}
	if w, _ := g.do("POST", "/ops/v1/cases/"+id+"/confirm", "", tokMod1, nil); w.Code != 403 {
		t.Fatalf("self-confirm: %d", w.Code)
	}
	// The second moderator can disagree: back to the queue, interim suspension lifted.
	w, out = g.do("POST", "/ops/v1/cases/"+id+"/confirm", "", tokMod2, map[string]any{"agree": false, "reason": "Messages read as a joke"})
	if w.Code != 200 || out["state"] != "open" || g.dir.get("bob") != "good" {
		t.Fatalf("reject: %d %s standing=%q", w.Code, w.Body, g.dir.get("bob"))
	}
	g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "uphold", "reason": "Two members describe the same blackmail"})
	w, out = g.do("POST", "/ops/v1/cases/"+id+"/confirm", "", tokMod2, nil)
	if w.Code != 200 || g.dir.get("bob") != "permanent" {
		t.Fatalf("confirm: %d %s", w.Code, w.Body)
	}
	if d := out["decision"].(map[string]any); d["decided_by"] != "mod1" || d["confirmed_by"] != "mod2" || d["retain_anchors"] != true {
		t.Fatalf("decision: %v", d)
	}
}

func TestAppealNeedsDifferentReviewerAndRestores(t *testing.T) {
	g := newRig(t, store.NewMemory())
	g.report("alice", "th_ab", "threat_violence")
	id := g.report("carol", "th_cb", "threat_violence")
	if g.dir.get("bob") != "suspend" {
		t.Fatalf("threat of violence not frozen automatically: %q", g.dir.get("bob"))
	}
	g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "uphold", "reason": "Threats of violence in both chats"})
	g.do("POST", "/ops/v1/cases/"+id+"/confirm", "", tokMod2, nil)
	_, st := g.do("GET", "/v1/me/standing", "bob", "", nil)
	dec := st["decisions"].([]any)[0].(map[string]any)["decision_id"].(string)

	if w, _ := g.do("POST", "/v1/appeals", "bob", "", map[string]any{"decision_id": dec, "text": "short"}); w.Code != 422 {
		t.Fatalf("short appeal: %d", w.Code)
	}
	if w, _ := g.do("POST", "/v1/appeals", "alice", "", map[string]any{"decision_id": dec, "text": "This isn't my decision to appeal"}); w.Code != 404 {
		t.Fatalf("appeal by someone else: %d", w.Code)
	}
	if w, _ := g.do("POST", "/v1/appeals", "bob", "", map[string]any{"decision_id": dec, "text": "Those messages were quoted from a film we both like."}); w.Code != 201 {
		t.Fatalf("appeal: %d", w.Code)
	}
	if w, _ := g.do("POST", "/v1/appeals", "bob", "", map[string]any{"decision_id": dec, "text": "Those messages were quoted from a film we both like."}); w.Code != 409 {
		t.Fatalf("second appeal: %d", w.Code)
	}
	_, q := g.do("GET", "/ops/v1/cases", "", tokLead, nil)
	if l := q["cases"].([]any); len(l) != 1 || !l[0].(map[string]any)["appeal_pending"].(bool) {
		t.Fatalf("appeal not queued: %v", l)
	}
	for _, tok := range []string{tokMod1, tokMod2} {
		if w, _ := g.do("POST", "/ops/v1/cases/"+id+"/appeal", "", tok, map[string]any{"member_wins": true, "reason": "Context shows a quote"}); w.Code != 403 {
			t.Fatalf("involved reviewer %s: %d", tok[:4], w.Code)
		}
	}
	w, _ := g.do("POST", "/ops/v1/cases/"+id+"/appeal", "", tokLead, map[string]any{"member_wins": true, "reason": "Context shows a quote"})
	if w.Code != 200 || g.dir.get("bob") != "good" {
		t.Fatalf("overturn: %d standing=%q", w.Code, g.dir.get("bob"))
	}
	_, st = g.do("GET", "/v1/me/standing", "bob", "", nil)
	if st["standing"] != "good" {
		t.Fatalf("standing after overturn: %v", st["standing"])
	}
}

func TestGuardianSignalOpensCase(t *testing.T) {
	g := newRig(t, store.NewMemory())
	g.svc.FromSignal("th_ab", "alice", "bob", "child_safety")
	g.svc.FromSignal("th_ab", "alice", "bob", "nothing")
	for i := 0; i < 100 && g.dir.get("bob") != "suspend"; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	if g.dir.get("bob") != "suspend" {
		t.Fatal("child-safety signal didn't freeze the subject")
	}
	_, q := g.do("GET", "/ops/v1/cases", "", tokMod1, nil)
	l := q["cases"].([]any)
	if len(l) != 1 || l[0].(map[string]any)["source"] != "guardian" || l[0].(map[string]any)["category"] != "minor" {
		t.Fatalf("queue: %v", l)
	}
	// A human confirming the on-device signal is a ground (G5): no second reporter needed.
	id := l[0].(map[string]any)["id"].(string)
	w, out := g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "uphold", "reason": "Member stated they are 16"})
	if w.Code != 200 || out["state"] != "pending_second" {
		t.Fatalf("decision: %d %s", w.Code, w.Body)
	}
}

func TestAuditChainDetectsTampering(t *testing.T) {
	g := newRig(t, store.NewMemory())
	id := g.report("alice", "th_ab", "romance_fraud")
	g.do("GET", "/ops/v1/cases/"+id, "", tokMod1, nil)
	g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "dismiss", "reason": "Refund was agreed"})
	_, out := g.do("GET", "/ops/v1/audit/verify", "", tokMod1, nil)
	if out["ok"] != true || out["entries"].(float64) != 2 {
		t.Fatalf("verify: %v", out)
	}
	chain := append([]AuditEntry(nil), g.svc.chain...)
	chain[0].Detail = "edited"
	if err := VerifyChain(chain); !errors.Is(err, ErrTampered) {
		t.Fatalf("edit not detected: %v", err)
	}
	if err := VerifyChain(g.svc.chain[1:]); !errors.Is(err, ErrTampered) {
		t.Fatalf("deletion not detected: %v", err)
	}
	_, stats := g.do("GET", "/ops/v1/stats", "", tokMod1, nil)
	if stats["decided"].(float64) != 1 || strings.Contains(mustJSON(stats), "alice") {
		t.Fatalf("stats: %v", stats)
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func restart(t *testing.T, st store.Store) {
	g := newRig(t, st)
	g.report("alice", "th_ab", "harassment")
	id := g.report("carol", "th_cb", "harassment")
	g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "uphold", "reason": "Pressure after a clear no"})

	g2 := newRig(t, st)
	_, q := g2.do("GET", "/ops/v1/cases", "", tokMod1, nil)
	if len(q["cases"].([]any)) != 0 {
		t.Fatalf("open cases after restart: %v", q)
	}
	_, out := g2.do("GET", "/ops/v1/audit/verify", "", tokMod1, nil)
	if out["ok"] != true || out["entries"].(float64) != 2 {
		t.Fatalf("audit after restart: %v", out)
	}
	_, st2 := g2.do("GET", "/v1/me/standing", "bob", "", nil)
	if st2["standing"] != "warn" {
		t.Fatalf("standing after restart: %v", st2)
	}
}

func TestRestartMemory(t *testing.T) { restart(t, store.NewMemory()) }

func TestRestartPostgres(t *testing.T) {
	dsn := os.Getenv("TRYST_TEST_PG")
	if dsn == "" {
		t.Skip("TRYST_TEST_PG not set")
	}
	p, err := store.OpenPostgres(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ns := make([]byte, 6)
	_, _ = rand.Read(ns)
	restart(t, prefixed{p, "t" + hex.EncodeToString(ns) + ":"})
}

func TestExportAndErase(t *testing.T) {
	g := newRig(t, store.NewMemory())
	ctx := context.Background()
	// bob reported alice (closed with a warning) and carol reported bob (still open).
	g.dir.threads["th_ba"] = [2]string{"bob", "alice"}
	ref := g.report("bob", "th_ba", "harassment")
	open := g.report("carol", "th_cb", "harassment")
	byAlice := g.report("alice", "th_ab", "other")
	g.do("POST", "/ops/v1/cases/"+byAlice+"/decision", "", tokMod1, map[string]any{"outcome": "uphold", "reason": "Repeated unwanted messages"})

	ex, _ := g.svc.Export(ctx, "bob")
	m := ex.(map[string]any)
	var back map[string][]any
	_ = json.Unmarshal([]byte(mustJSON(m)), &back)
	if len(back["reports_you_made"]) != 1 || len(back["decisions_about_you"]) != 1 {
		t.Fatalf("export: %v", back)
	}
	if s := mustJSON(m); !strings.Contains(s, "Repeated unwanted messages") || strings.Contains(s, "carol") || strings.Contains(s, "alice") {
		t.Fatalf("export content: %s", s)
	}

	rc, err := g.svc.Erase(ctx, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(rc.Retained) != 1 || !strings.Contains(rc.Retained[0], "under review") {
		t.Fatalf("receipt: %+v", rc)
	}
	g.svc.mu.Lock()
	reportByBob := g.svc.cases[ref]
	_, openKept := g.svc.cases[open]
	_, warnKept := g.svc.cases[byAlice]
	g.svc.mu.Unlock()
	if reportByBob == nil || !strings.HasPrefix(reportByBob.Reporter, "erased:") || reportByBob.Disclosed != nil {
		t.Fatalf("bob's report not de-identified: %+v", reportByBob)
	}
	if !openKept || warnKept {
		t.Fatalf("open kept=%v, closed warning kept=%v", openKept, warnKept)
	}
	if err := VerifyChain(g.svc.chain); err != nil {
		t.Fatal(err)
	}
}

func TestDecisionNotices(t *testing.T) {
	g := newRig(t, store.NewMemory())
	var mu sync.Mutex
	var got []string
	g.svc.Notify = func(acc, kind, _ string) { mu.Lock(); got = append(got, acc+":"+kind); mu.Unlock() }
	id := g.report("alice", "th_ab", "harassment")
	g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "uphold", "reason": "Pressure after a clear no"})
	_, st := g.do("GET", "/v1/me/standing", "bob", "", nil)
	dec := st["decisions"].([]any)[0].(map[string]any)["decision_id"].(string)
	g.do("POST", "/v1/appeals", "bob", "", map[string]any{"decision_id": dec, "text": "It was a misunderstanding about timing."})
	g.do("POST", "/ops/v1/cases/"+id+"/appeal", "", tokLead, map[string]any{"member_wins": false, "reason": "The messages are clear"})
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(got, ",") != "bob:safety,bob:appeal" {
		t.Fatalf("notices: %v", got)
	}
}

func TestQueueStats(t *testing.T) {
	g := newRig(t, store.NewMemory())
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	g.svc.now = func() time.Time { return now }
	g.report("alice", "th_ab", "minor")
	g.report("carol", "th_cb", "harassment")
	if o, od, u := g.svc.QueueStats(); o != 2 || od != 0 || u != 0 {
		t.Fatalf("fresh: %d %d %d", o, od, u)
	}
	now = now.Add(20 * time.Minute)
	if o, od, u := g.svc.QueueStats(); o != 2 || od != 1 || u != 1 {
		t.Fatalf("minor overdue: %d %d %d", o, od, u)
	}
}
