package identity

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

// phoneMember signs up on a phone and returns the passkey and a full-session token.
func (g *assuranceRig) phoneMember(contact string) (*softAuth, string) {
	phone := newSoftAuth(g.h.t, "platform")
	g.h.register(g.h.signUp(contact), uaIPhone, phone)
	return phone, g.login(phone)
}

func (g *assuranceRig) login(phone *softAuth) string {
	loginID, _ := g.h.firstFactor(phone, uaIPhone)
	r := g.h.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": loginID}, "", uaIPhone)
	g.h.mustStatus(r, 200)
	return r.json(g.h.t)["access_token"].(string)
}

// stepUp repeats both checks inside the session; it returns the final status.
func (g *assuranceRig) stepUp(tok string, phone *softAuth) int {
	r := g.h.do("POST", "/v1/auth/step-up", nil, tok, uaIPhone)
	g.h.mustStatus(r, 200)
	var begin struct {
		LoginID string          `json:"login_id"`
		Options json.RawMessage `json:"options"`
	}
	_ = json.Unmarshal(r.body, &begin)
	cred, _ := phone.get(begin.Options, false)
	r = g.h.do("POST", "/v1/auth/login/passkey", map[string]any{"login_id": begin.LoginID, "credential": cred}, "", uaIPhone)
	if r.status != 200 {
		return r.status
	}
	return g.h.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": begin.LoginID}, "", uaIPhone).status
}

func TestStepUpGuardsDevices(t *testing.T) {
	g := newAssuranceRig(t, store.NewMemory(), DevAssurance{})
	phone, tok := g.phoneMember("sam@example.com")
	other, _ := g.phoneMember("kit@example.com")

	g.h.mustStatus(g.h.do("GET", "/v1/auth/devices", nil, tok, uaIPhone), 403)
	g.h.mustStatus(g.h.do("POST", "/v1/auth/passkeys/register/begin", nil, tok, uaIPhone), 403)
	if a, _ := g.account(tok); a.StepUp {
		t.Fatal("step-up without the checks")
	}
	// Someone else's passkey cannot step up this session.
	if c := g.stepUp(tok, other); c != 401 {
		t.Fatalf("other member's passkey: %d", c)
	}
	if c := g.stepUp(tok, phone); c != 200 {
		t.Fatalf("step-up: %d", c)
	}
	if a, _ := g.account(tok); !a.StepUp {
		t.Fatal("step-up not reported")
	}
	r := g.h.do("GET", "/v1/auth/devices", nil, tok, uaIPhone)
	g.h.mustStatus(r, 200)
	devs := r.json(t)["devices"].([]any)
	if len(devs) != 1 {
		t.Fatalf("devices: %v", devs)
	}
	only := devs[0].(map[string]any)["id"].(string)
	g.h.mustStatus(g.h.do("DELETE", "/v1/auth/devices/"+only, nil, tok, uaIPhone), 409) // last device

	// Adding a second device now works, and then the first can be removed.
	g.h.register(tok, uaIPhone, newSoftAuth(t, "platform"))
	g.h.mustStatus(g.h.do("DELETE", "/v1/auth/devices/"+only, nil, tok, uaIPhone), 204)

	for range 4 { // phone sessions idle out after 2 minutes; keep this one alive
		g.h.clock.add(90 * time.Second)
		g.h.mustStatus(g.h.me(tok), 200)
	}
	g.h.mustStatus(g.h.do("GET", "/v1/auth/devices", nil, tok, uaIPhone), 403)
	if r := g.h.do("POST", "/v1/burn", nil, tok, uaIPhone); r.status != 204 {
		t.Fatalf("Burn must never need step-up: %d", r.status)
	}
}

// recoveryCheck runs one provider check for a recovery, as the recovering browser.
func (g *assuranceRig) recoveryCheck(rid, kind string, res AssuranceResult) string {
	r := g.h.do("POST", "/v1/auth/recovery/"+rid+"/check", map[string]string{"kind": kind}, "", uaIPhone)
	g.h.mustStatus(r, 201)
	m := r.json(g.h.t)
	u, _ := url.Parse(m["url"].(string))
	if u.Query().Get("recovery") != rid {
		g.h.t.Fatalf("dev URL lacks the recovery: %s", u)
	}
	res.SessionID = m["session_id"].(string)
	// Without the recovery ID, the dev stand-in refuses.
	g.h.mustStatus(g.h.do("POST", "/v1/assurance/dev/complete", res, "", uaIPhone), 404)
	r = g.h.do("POST", "/v1/assurance/dev/complete?recovery="+url.QueryEscape(rid), res, "", uaIPhone)
	g.h.mustStatus(r, 200)
	return r.json(g.h.t)["state"].(string)
}

func (g *assuranceRig) beginRecovery(contact string) (string, map[string]any) {
	r := g.h.do("POST", "/v1/auth/recovery/begin", map[string]string{"contact": contact}, "", "")
	g.h.mustStatus(r, 202)
	m := r.json(g.h.t)
	return m["recovery_id"].(string), m
}

func (g *assuranceRig) recoveryState(rid string) map[string]any {
	r := g.h.do("GET", "/v1/auth/recovery/"+rid, nil, "", "")
	g.h.mustStatus(r, 200)
	return r.json(g.h.t)
}

type noticeLog struct {
	mu   sync.Mutex
	list []string
}

func (n *noticeLog) add(_, kind, _ string) { n.mu.Lock(); n.list = append(n.list, kind); n.mu.Unlock() }
func (n *noticeLog) all() string           { n.mu.Lock(); defer n.mu.Unlock(); return strings.Join(n.list, ",") }

func TestRecoveryNeedsSameDocumentAndHold(t *testing.T) {
	st := store.NewMemory()
	g := newAssuranceRig(t, st, DevAssurance{})
	notes := &noticeLog{}
	g.srv.cfg.Notify = notes.add
	phone, tok := g.phoneMember("ana@example.com")
	if s := g.verify(tok, "1990-05", "P1234567"); s != "passed" {
		t.Fatal(s)
	}

	// Unknown contacts get the same answer, and their recovery never completes.
	ghost, gm := g.beginRecovery("nobody@example.com")
	_, am := g.beginRecovery("ana@example.com")
	if len(gm) != len(am) {
		t.Fatalf("responses differ: %v vs %v", gm, am)
	}
	if s := g.recoveryCheck(ghost, "liveness", AssuranceResult{Result: "pass"}); s != "failed" {
		t.Fatalf("ghost recovery: %s", s)
	}

	// The wrong document ends the attempt.
	rid, _ := g.beginRecovery("ana@example.com")
	g.recoveryCheck(rid, "liveness", AssuranceResult{Result: "pass"})
	if s := g.recoveryCheck(rid, "age", AssuranceResult{Result: "pass", IsAdult: true, BirthYM: "1990-05", DocumentNumber: "X999"}); s != "document_mismatch" {
		t.Fatalf("mismatch: %s", s)
	}
	if s := g.recoveryState(rid)["state"]; s != "failed" {
		t.Fatalf("after mismatch: %v", s)
	}

	rid, _ = g.beginRecovery("ana@example.com")
	g.h.mustStatus(g.h.do("POST", "/v1/auth/recovery/"+rid+"/check", map[string]string{"kind": "age"}, "", ""), 409)
	g.recoveryCheck(rid, "liveness", AssuranceResult{Result: "pass"})
	if s := g.recoveryCheck(rid, "age", AssuranceResult{Result: "pass", IsAdult: true, BirthYM: "1990-05", DocumentNumber: "P1234567"}); s != "passed" {
		t.Fatal(s)
	}
	if s := g.recoveryState(rid)["state"]; s != "hold" {
		t.Fatalf("state: %v", s)
	}
	if n := notes.all(); !strings.Contains(n, "recovery_started") || !strings.Contains(n, "recovery_hold") {
		t.Fatalf("member not told: %s", n)
	}
	if g.h.me(tok).json(t)["recovery_pending"] == nil {
		t.Fatal("signed-in device can't see the pending recovery")
	}
	g.h.mustStatus(g.h.do("POST", "/v1/auth/recovery/"+rid+"/finish", nil, "", ""), 409)
	if a, _ := g.account(tok); a.Tier.String() != "V2" {
		t.Fatalf("recovery changed the tier: %v", a.Tier)
	}

	// After the hold (and a restart), every old passkey and session goes.
	g.h.clock.add(RecoveryHold + time.Minute)
	g2, err := New(g.srv.cfg, NewStore())
	if err != nil {
		t.Fatal(err)
	}
	g.h.srv = httptest.NewServer(g2)
	t.Cleanup(g.h.srv.Close)
	g.srv = g2
	r := g.h.do("POST", "/v1/auth/recovery/"+rid+"/finish", nil, "", "")
	g.h.mustStatus(r, 200)
	rtok := r.json(t)["access_token"].(string)
	g.h.mustStatus(g.h.do("POST", "/v1/auth/recovery/"+rid+"/finish", nil, "", ""), 409) // once only
	g.h.mustStatus(g.h.me(tok), 401)
	lr := g.h.do("POST", "/v1/auth/login/begin", nil, "", uaIPhone)
	var begin struct {
		LoginID string          `json:"login_id"`
		Options json.RawMessage `json:"options"`
	}
	_ = json.Unmarshal(lr.body, &begin)
	cred, _ := phone.get(begin.Options, false)
	g.h.mustStatus(g.h.do("POST", "/v1/auth/login/passkey", map[string]any{"login_id": begin.LoginID, "credential": cred}, "", uaIPhone), 401)

	// The recovery session registers a new passkey, which then signs in as normal.
	g.h.mustStatus(g.h.do("GET", "/v1/auth/devices", nil, rtok, uaIPhone), 401)
	fresh := newSoftAuth(t, "platform")
	g.h.register(rtok, uaIPhone, fresh)
	if a, ok := g.account(g.login(fresh)); !ok || a.BirthYM != "1990-05" {
		t.Fatalf("recovered account: %+v %v", a, ok)
	}
	g.h.clock.add(recoverySessionTTL + time.Minute)
	g.h.mustStatus(g.h.do("POST", "/v1/auth/passkeys/register/begin", nil, rtok, uaIPhone), 401)
}

func TestRecoveryCanBeCancelledFromAnyDevice(t *testing.T) {
	g := newAssuranceRig(t, store.NewMemory(), DevAssurance{})
	phone, tok := g.phoneMember("bo@example.com")
	g.verify(tok, "1988-01", "D7654321")
	rid, _ := g.beginRecovery("bo@example.com")
	g.recoveryCheck(rid, "liveness", AssuranceResult{Result: "pass"})
	g.recoveryCheck(rid, "age", AssuranceResult{Result: "pass", IsAdult: true, BirthYM: "1988-01", DocumentNumber: "D7654321"})

	g.h.mustStatus(g.h.do("POST", "/v1/auth/recovery/cancel", nil, "", ""), 401)
	r := g.h.do("POST", "/v1/auth/recovery/cancel", nil, tok, uaIPhone) // no step-up needed
	g.h.mustStatus(r, 200)
	if r.json(t)["cancelled"].(float64) != 1 {
		t.Fatalf("cancel: %s", r.body)
	}
	g.h.clock.add(RecoveryHold + time.Minute)
	g.h.mustStatus(g.h.do("POST", "/v1/auth/recovery/"+rid+"/finish", nil, "", ""), 409)
	g.h.mustStatus(g.h.me(g.login(phone)), 200) // the passkey still works

	// Begins are limited per contact (the first was over a day ago).
	for range recoveryMaxPerDay {
		g.beginRecovery("bo@example.com")
	}
	g.h.mustStatus(g.h.do("POST", "/v1/auth/recovery/begin", map[string]string{"contact": "bo@example.com"}, "", ""), 429)
}
