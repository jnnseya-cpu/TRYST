package identity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/gate"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

// signedProvider is a stand-in production provider whose callbacks carry a shared token.
type signedProvider struct{}

func (signedProvider) Name() string { return "test" }
func (signedProvider) Start(_ context.Context, a AssuranceSession, _ string) (string, error) {
	return "https://provider.example/s/" + a.ID, nil
}
func (signedProvider) VerifyCallback(r *http.Request, _ []byte) error {
	if r.Header.Get("X-Test-Provider") != "genuine" {
		return errors.New("bad signature")
	}
	return nil
}

type assuranceRig struct {
	h   *harness
	srv *Server
	st  store.Store
}

func newAssuranceRig(t *testing.T, st store.Store, p AssuranceProvider) *assuranceRig {
	ck := &clock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	s, err := New(Config{RPID: "localhost", RPDisplayName: "Member services", RPOrigins: []string{"http://localhost:3000"},
		Env: "dev", ContactPepper: []byte("dev-pepper"), Liveness: DevLiveness{}, Now: ck.now, Persist: st, Assurance: p, RateLimitPerMin: 1000}, NewStore())
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, srv: httptest.NewServer(s), clock: ck}
	t.Cleanup(h.srv.Close)
	return &assuranceRig{h, s, st}
}

// member signs up and signs in on a phone, returning a full-session token.
func (g *assuranceRig) member(contact string) string {
	phone := newSoftAuth(g.h.t, "platform")
	g.h.register(g.h.signUp(contact), uaIPhone, phone)
	loginID, _ := g.h.firstFactor(phone, uaIPhone)
	r := g.h.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": loginID}, "", uaIPhone)
	g.h.mustStatus(r, 200)
	return r.json(g.h.t)["access_token"].(string)
}

func (g *assuranceRig) start(tok, kind string) (int, string) {
	r := g.h.do("POST", "/v1/assurance/session", map[string]string{"kind": kind}, tok, uaIPhone)
	if r.status != 201 {
		return r.status, ""
	}
	return r.status, r.json(g.h.t)["session_id"].(string)
}

func (g *assuranceRig) complete(tok, sid string, res AssuranceResult) string {
	res.SessionID = sid
	r := g.h.do("POST", "/v1/assurance/dev/complete", res, tok, uaIPhone)
	g.h.mustStatus(r, 200)
	return r.json(g.h.t)["state"].(string)
}

func (g *assuranceRig) account(tok string) (Account, bool) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	return g.srv.Authenticate(req)
}

// verify takes a member from V0 to V2.
func (g *assuranceRig) verify(tok, birth, doc string) string {
	_, sid := g.start(tok, "liveness")
	g.complete(tok, sid, AssuranceResult{Result: "pass"})
	g.h.do("POST", "/v1/attestations/conduct", map[string]any{"version": ConductVersion, "accept": true}, tok, uaIPhone)
	_, sid = g.start(tok, "age")
	return g.complete(tok, sid, AssuranceResult{Result: "pass", IsAdult: true, BirthYM: birth, DocumentNumber: doc})
}

func TestVerificationToV2(t *testing.T) {
	g := newAssuranceRig(t, store.NewMemory(), DevAssurance{})
	tok := g.member("ana@example.com")
	if c, _ := g.start(tok, "age"); c != 409 {
		t.Fatalf("ID check before the face check: %d", c)
	}
	c, sid := g.start(tok, "liveness")
	if c != 201 {
		t.Fatalf("liveness: %d", c)
	}
	if st := g.complete(tok, sid, AssuranceResult{Result: "pass"}); st != "passed" {
		t.Fatal(st)
	}
	if a, _ := g.account(tok); a.Tier != gate.V1 {
		t.Fatalf("tier after liveness: %v", a.Tier)
	}
	_, sid = g.start(tok, "age")
	if st := g.complete(tok, sid, AssuranceResult{Result: "pass", IsAdult: true, BirthYM: "1990-05", DocumentNumber: "P1234567"}); st != "passed" {
		t.Fatal(st)
	}
	if a, _ := g.account(tok); a.Tier != gate.V1 || a.BirthYM != "1990-05" {
		t.Fatalf("V2 granted before the conduct declaration: %+v", a)
	}
	if r := g.h.do("POST", "/v1/attestations/conduct", map[string]any{"version": "old", "accept": true}, tok, uaIPhone); r.status != 422 {
		t.Fatalf("stale declaration accepted: %d", r.status)
	}
	g.h.mustStatus(g.h.do("POST", "/v1/attestations/conduct", map[string]any{"version": ConductVersion, "accept": true}, tok, uaIPhone), 200)
	if a, _ := g.account(tok); a.Tier != gate.V2 {
		t.Fatalf("tier: %v", a.Tier)
	}
	// A result is applied once: replaying it changes nothing.
	if st := g.complete(tok, sid, AssuranceResult{Result: "fail"}); st != "passed" {
		t.Fatalf("replayed result changed the outcome: %s", st)
	}
	st := g.h.do("GET", "/v1/assurance/status", nil, tok, uaIPhone).json(t)
	if st["tier"] != "V2" || st["id_checked"] != true {
		t.Fatalf("status: %v", st)
	}
}

func TestUnder18IsRefusedAndLockedOut(t *testing.T) {
	g := newAssuranceRig(t, store.NewMemory(), DevAssurance{})
	tok := g.member("teen@example.com")
	// Born this month 18 years ago: TRYST's own check waits until next month.
	birth := g.h.clock.now().AddDate(-18, 0, 0).Format("2006-01")
	if st := g.verify(tok, birth, "D999"); st != "refused" {
		t.Fatalf("state: %s", st)
	}
	if _, ok := g.account(tok); ok {
		t.Fatal("refused account still signed in")
	}
	r := g.h.do("POST", "/v1/auth/login/begin", nil, "", uaIPhone)
	g.h.mustStatus(r, 200)
	// The provider saying "adult" isn't enough on its own, and neither is the date.
	tok2 := g.member("teen2@example.com")
	_, sid := g.start(tok2, "liveness")
	g.complete(tok2, sid, AssuranceResult{Result: "pass"})
	_, sid = g.start(tok2, "age")
	if st := g.complete(tok2, sid, AssuranceResult{Result: "pass", IsAdult: false, BirthYM: "1980-01", DocumentNumber: "D1"}); st != "refused" {
		t.Fatalf("provider said minor: %s", st)
	}
}

func TestOnePersonOneAccountAndBans(t *testing.T) {
	st := store.NewMemory()
	g := newAssuranceRig(t, st, DevAssurance{})
	a := g.member("first@example.com")
	if s := g.verify(a, "1985-03", "P-SAME"); s != "passed" {
		t.Fatal(s)
	}
	b := g.member("second@example.com")
	if s := g.verify(b, "1985-03", "p-same "); s != "duplicate" {
		t.Fatalf("second account on the same document: %s", s)
	}
	// After a restart the index still knows the document.
	g2 := newAssuranceRig(t, st, DevAssurance{})
	c := g2.member("third@example.com")
	if s := g2.verify(c, "1985-03", "P-SAME"); s != "duplicate" {
		t.Fatalf("after restart: %s", s)
	}
	// A permanently removed member who deletes their account can't come back.
	acc, _ := g2.account(a)
	g2.srv.BanMember(context.Background(), acc.ID, true)
	if _, err := g2.srv.Erase(context.Background(), acc.ID); err != nil {
		t.Fatal(err)
	}
	d := g2.member("fourth@example.com")
	if s := g2.verify(d, "1985-03", "P-SAME"); s != "not_eligible" {
		t.Fatalf("banned document: %s", s)
	}
}

func TestProviderCallbacksMustBeSigned(t *testing.T) {
	g := newAssuranceRig(t, store.NewMemory(), signedProvider{})
	tok := g.member("cb@example.com")
	r := g.h.do("POST", "/v1/assurance/session", map[string]string{"kind": "liveness"}, tok, uaIPhone)
	g.h.mustStatus(r, 201)
	sid := r.json(t)["session_id"].(string)
	if r.json(t)["url"] != "https://provider.example/s/"+sid {
		t.Fatal("hand-off url")
	}
	if r := g.h.do("POST", "/v1/assurance/callback", map[string]string{"session_id": sid, "result": "pass"}, "", ""); r.status != 401 {
		t.Fatalf("unsigned callback: %d", r.status)
	}
	if r := g.h.do("POST", "/v1/assurance/dev/complete", map[string]string{"session_id": sid, "result": "pass"}, tok, uaIPhone); r.status != 404 {
		t.Fatalf("dev completion with a real provider: %d", r.status)
	}
	g.h.mustStatus(g.h.do("POST", "/v1/assurance/callback", map[string]string{"session_id": sid, "result": "pass"}, "", "", "X-Test-Provider", "genuine"), 204)
	if a, _ := g.account(tok); a.Tier != gate.V1 {
		t.Fatalf("tier: %v", a.Tier)
	}
	if _, err := New(Config{RPID: "x", RPDisplayName: "x", RPOrigins: []string{"https://x"}, Env: "prod", ContactPepper: make([]byte, 32),
		AnchorPepper: make([]byte, 32), Assurance: DevAssurance{}}, NewStore()); err == nil {
		t.Fatal("dev assurance accepted in prod")
	}
}

func TestFaceCheckNeedsBiometricConsent(t *testing.T) {
	g := newAssuranceRig(t, store.NewMemory(), DevAssurance{})
	given := map[string]bool{}
	g.srv.cfg.ConsentGranted = func(account, kind string) bool { return kind == "biometric_login" && given[account] }
	tok := g.member("cy@example.com")
	r := g.h.do("POST", "/v1/assurance/session", map[string]string{"kind": "liveness"}, tok, uaIPhone)
	if r.status != 403 || r.json(t)["consent_kind"] != "biometric_login" {
		t.Fatalf("without consent: %d %s", r.status, r.body)
	}
	a, _ := g.account(tok)
	given[a.ID] = true
	if c, _ := g.start(tok, "liveness"); c != 201 {
		t.Fatalf("with consent: %d", c)
	}
}
