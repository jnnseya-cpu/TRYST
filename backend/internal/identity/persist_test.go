package identity

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

func harnessOn(t *testing.T, st store.Store, ck *clock) *harness {
	t.Helper()
	s, err := New(Config{
		RPID: "localhost", RPDisplayName: "Member services", RPOrigins: []string{"http://localhost:3000"},
		Env: "dev", ContactPepper: []byte("dev-pepper"), Liveness: DevLiveness{}, Now: ck.now, Persist: st,
	}, NewStore())
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, srv: httptest.NewServer(s), clock: ck}
	t.Cleanup(h.srv.Close)
	return h
}

// restartSurvives: sign up and sign in, restart on the same store, and the member can still
// sign in with the same passkey; their session and security activity are still there; an OTP
// cannot add a passkey to the restored account; Burn is saved too.
func restartSurvives(t *testing.T, st store.Store) {
	ck := &clock{t: time.Now()}
	h := harnessOn(t, st, ck)
	phone := newSoftAuth(t, "platform")
	onb := h.signUp("restart@example.com")
	h.register(onb, uaIPhone, phone)
	loginID, _ := h.firstFactor(phone, uaIPhone)
	r := h.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": loginID}, "", uaIPhone)
	h.mustStatus(r, 200)
	tok := r.json(t)["access_token"].(string)
	h.srv.Close()

	h2 := harnessOn(t, st, ck) // a new process on the same database
	h2.mustStatus(h2.me(tok), 200)
	sec := h2.do("GET", "/v1/me/security", nil, tok, "")
	h2.mustStatus(sec, 200)
	if tot := sec.json(t)["totals"].(map[string]any); tot["login_ok"] == nil || tot["passkey_added"] == nil {
		t.Fatalf("security activity lost: %v", tot)
	}
	loginID, _ = h2.firstFactor(phone, uaIPhone)
	r = h2.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": loginID}, "", uaIPhone)
	h2.mustStatus(r, 200)
	// The restored account is sealed: a sign-up code can't add a passkey.
	r = h2.do("POST", "/v1/auth/start", map[string]string{"contact": "restart@example.com"}, "", "")
	otp := r.header.Get("X-Dev-OTP")
	r = h2.do("POST", "/v1/auth/verify", map[string]string{"contact": "restart@example.com", "otp": otp}, "", "")
	if r.status != 409 {
		t.Fatalf("existing account after restart: %d", r.status)
	}
	// Burn is persisted: after another restart the old session is gone.
	h2.mustStatus(h2.do("POST", "/v1/burn", nil, tok, ""), 204)
	h2.srv.Close()
	h3 := harnessOn(t, st, ck)
	if r := h3.me(tok); r.status != 401 {
		t.Fatalf("burned session survived a restart: %d", r.status)
	}
}

func TestRestartWithMemoryStore(t *testing.T) { restartSurvives(t, store.NewMemory()) }

func TestRestartWithPostgres(t *testing.T) {
	dsn := os.Getenv("TRYST_TEST_PG")
	if dsn == "" {
		t.Skip("TRYST_TEST_PG not set")
	}
	p, err := store.OpenPostgres(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	all, _ := p.All(context.Background(), "identity")
	for id := range all {
		p.Delete(context.Background(), "identity", id)
	}
	restartSurvives(t, p)
}
