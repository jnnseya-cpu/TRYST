package safetysvc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/store"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcauth"
)

func TestStandingSurvivesMemberPlatformOutageAndRestart(t *testing.T) {
	st := store.NewMemory()
	g := newRig(t, st)
	g.dir.down = true
	g.report("alice", "th_ab", "threat_violence") // freezes bob automatically
	if g.dir.get("bob") != "" || g.svc.OutboxPending(context.Background()) != 1 {
		t.Fatalf("expected a queued standing, got applied=%q pending=%d", g.dir.get("bob"), g.svc.OutboxPending(context.Background()))
	}
	// The safety service restarts while the member platform is still down.
	g2 := newRig(t, st)
	g2.dir.down = false
	if left := g2.svc.Flush(context.Background()); left != 0 {
		t.Fatalf("left = %d", left)
	}
	if g2.dir.get("bob") != "suspend" {
		t.Fatalf("standing after recovery = %q", g2.dir.get("bob"))
	}
}

func TestMandatoryExternalReport(t *testing.T) {
	g := newRig(t, store.NewMemory())
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	g.svc.now = func() time.Time { return now }
	g.svc.FromSignal("th_ab", "alice", "bob", "child_safety")
	_, q := g.do("GET", "/ops/v1/cases", "", tokMod1, nil)
	id := q["cases"].([]any)[0].(map[string]any)["id"].(string)
	g.do("POST", "/ops/v1/cases/"+id+"/decision", "", tokMod1, map[string]any{"outcome": "uphold", "reason": "Stated they are 15 years old"})
	g.do("POST", "/ops/v1/cases/"+id+"/confirm", "", tokMod2, nil)
	if c, out := g.do("POST", "/ops/v1/cases/"+id+"/external-report", "", tokMod1, map[string]any{"agency": "FBI", "reference": "X1"}); c.Code != 422 {
		t.Fatalf("bad agency: %d %v", c.Code, out)
	}
	// Decided, but the NCA report keeps it in the queue, and it becomes urgent after 24 h.
	_, q = g.do("GET", "/ops/v1/cases", "", tokMod1, nil)
	row := q["cases"].([]any)[0].(map[string]any)
	if row["state"] != "closed" || row["external_report_pending"] != true {
		t.Fatalf("queue row: %v", row)
	}
	now = now.Add(25 * time.Hour)
	if _, _, u := g.svc.QueueStats(); u != 1 {
		t.Fatalf("overdue external report not urgent: %d", u)
	}
	c, out := g.do("POST", "/ops/v1/cases/"+id+"/external-report", "", tokMod1, map[string]any{"agency": "NCA", "reference": "NCA-2026-000123"})
	if c.Code != 200 || out["external"].(map[string]any)["by"] != "mod1" {
		t.Fatalf("record: %d %v", c.Code, out)
	}
	if c, _ := g.do("POST", "/ops/v1/cases/"+id+"/external-report", "", tokMod1, map[string]any{"agency": "NCA", "reference": "NCA-2026-000124"}); c.Code != 409 {
		t.Fatalf("second report: %d", c.Code)
	}
	_, q = g.do("GET", "/ops/v1/cases", "", tokMod1, nil)
	if len(q["cases"].([]any)) != 0 {
		t.Fatalf("still queued: %v", q)
	}
	if e := g.svc.chain[len(g.svc.chain)-1]; e.Action != "external_report" || !strings.Contains(e.Detail, "NCA-2026-000123") {
		t.Fatalf("audit: %+v", e)
	}
	_, stats := g.do("GET", "/ops/v1/stats", "", tokMod1, nil)
	if stats["external_reports"].(map[string]any)["NCA"].(float64) != 1 {
		t.Fatalf("stats: %v", stats)
	}
	// Nothing to report on an undecided case.
	id2 := g.report("alice", "th_ab", "harassment")
	if c, _ := g.do("POST", "/ops/v1/cases/"+id2+"/external-report", "", tokMod1, map[string]any{"agency": "police", "reference": "CAD123"}); c.Code != 409 {
		t.Fatalf("undecided: %d", c.Code)
	}
}

func TestSignalIsIdempotent(t *testing.T) {
	g := newRig(t, store.NewMemory())
	for i := 0; i < 3; i++ {
		if err := g.svc.Signal(context.Background(), "sg_same", "th_ab", "alice", "bob", "fraud"); err != nil {
			t.Fatal(err)
		}
	}
	_, q := g.do("GET", "/ops/v1/cases", "", tokMod1, nil)
	if len(q["cases"].([]any)) != 1 {
		t.Fatalf("duplicate deliveries opened %d cases", len(q["cases"].([]any)))
	}
}

// memberPlatform is a fake of the member platform's internal API that checks signatures.
func memberPlatform(t *testing.T, secret []byte, standing map[string]string, notices *[]string) *httptest.Server {
	m := http.NewServeMux()
	m.HandleFunc("POST /internal/v1/authenticate", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		switch in["authorization"] {
		case "Bearer tok-alice":
			json.NewEncoder(w).Encode(map[string]any{"account": "alice", "tier": 2})
		case "Bearer tok-bob":
			json.NewEncoder(w).Encode(map[string]any{"account": "bob", "tier": 2})
		default:
			w.WriteHeader(401)
		}
	})
	m.HandleFunc("GET /internal/v1/threads/{id}/counterpart", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "th_ab" && r.URL.Query().Get("member") == "alice" {
			json.NewEncoder(w).Encode(map[string]string{"member": "bob"})
			return
		}
		w.WriteHeader(404)
	})
	m.HandleFunc("GET /internal/v1/members/{id}/summary", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"name": "Bob", "area": "London"})
	})
	m.HandleFunc("GET /internal/v1/members/{id}/main-photo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("jpeg"))
	})
	m.HandleFunc("PUT /internal/v1/members/{id}/standing", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		standing[r.PathValue("id")] = in["standing"]
		w.WriteHeader(204)
	})
	m.HandleFunc("POST /internal/v1/notices", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		*notices = append(*notices, in["account"]+":"+in["kind"])
		w.WriteHeader(204)
	})
	v := &svcauth.Verifier{Secrets: map[string][]byte{"safety": secret}}
	srv := httptest.NewServer(v.Wrap(m))
	t.Cleanup(srv.Close)
	return srv
}

func TestStandaloneOverSignedInternalAPIs(t *testing.T) {
	toPlatform := bytes.Repeat([]byte{1}, 32)
	toSafety := bytes.Repeat([]byte{2}, 32)
	standing, notices := map[string]string{}, []string{}
	plat := memberPlatform(t, toPlatform, standing, &notices)
	pc := &svcauth.Client{BaseURL: plat.URL, Caller: "safety", Secret: toPlatform}
	svc, err := New(RemoteMembers{pc}, devStaff(), RemoteDirectory{pc}, store.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	svc.Notices = RemoteNotices(pc)
	pub := httptest.NewServer(svc)
	defer pub.Close()
	internal := httptest.NewServer((&svcauth.Verifier{Secrets: map[string][]byte{"profile": toSafety}}).Wrap(svc.InternalHandler()))
	defer internal.Close()

	call := func(method, path, auth string, body any) (int, map[string]any) {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, pub.URL+path, bytes.NewReader(b))
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		out := map[string]any{}
		json.Unmarshal(raw, &out)
		return resp.StatusCode, out
	}
	if c, _ := call("POST", "/v1/reports", "Bearer forged", map[string]any{"thread_id": "th_ab", "category": "harassment"}); c != 401 {
		t.Fatalf("forged token: %d", c)
	}
	c, out := call("POST", "/v1/reports", "Bearer tok-alice", map[string]any{"thread_id": "th_ab", "category": "stalking"})
	if c != 201 {
		t.Fatalf("report: %d %v", c, out)
	}
	if standing["bob"] != "suspend" {
		t.Fatalf("interim freeze not delivered: %v", standing)
	}
	c, out = call("GET", "/ops/v1/cases/"+out["reference"].(string), "Staff "+tokMod1, nil)
	if c != 200 || out["subject"].(map[string]any)["name"] != "Bob" {
		t.Fatalf("case via remote directory: %d %v", c, out)
	}

	// The member platform sends signals and runs data rights through the internal API.
	sc := &svcauth.Client{BaseURL: internal.URL, Caller: "profile", Secret: toSafety}
	for i := 0; i < 2; i++ {
		if err := sc.Do(context.Background(), "POST", "/internal/v1/signals", map[string]string{"id": "sg_1", "thread_id": "th_ab", "reporter": "alice", "subject": "bob", "kind": "fraud"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	part := RemotePart{sc}
	ex, err := part.Export(context.Background(), "bob")
	if err != nil || ex.(map[string]any)["standing"] != "suspend" {
		t.Fatalf("export: %v %v", ex, err)
	}
	rc, err := part.Erase(context.Background(), "alice")
	if err != nil || rc.Part != "safety" {
		t.Fatalf("erase: %+v %v", rc, err)
	}
	bad := &svcauth.Client{BaseURL: internal.URL, Caller: "profile", Secret: toPlatform}
	if err := bad.Do(context.Background(), "POST", "/internal/v1/erase", map[string]string{"account": "bob"}, nil); !svcauth.IsStatus(err, 401) {
		t.Fatalf("wrongly signed erase: %v", err)
	}
	_, q := call("GET", "/ops/v1/cases", "Staff "+tokMod1, nil)
	if n := len(q["cases"].([]any)); n != 2 {
		t.Fatalf("cases = %d, want 2 (one report, one de-duplicated signal)", n)
	}
}

func TestReportRateLimit(t *testing.T) {
	g := newRig(t, store.NewMemory())
	for i := 0; i < MaxReportsPerDay; i++ {
		g.report("alice", "th_ab", "other")
	}
	if w, _ := g.do("POST", "/v1/reports", "alice", "", map[string]any{"thread_id": "th_ab", "category": "other"}); w.Code != 429 {
		t.Fatalf("report %d: %d", MaxReportsPerDay+1, w.Code)
	}
}
