package identity

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

func TestExportAndErase(t *testing.T) {
	st := store.NewMemory()
	ck := &clock{t: time.Now()}
	s, err := New(Config{
		RPID: "localhost", RPDisplayName: "Member services", RPOrigins: []string{"http://localhost:3000"},
		Env: "dev", ContactPepper: []byte("dev-pepper"), Liveness: DevLiveness{}, Now: ck.now, Persist: st,
	}, NewStore())
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, srv: httptest.NewServer(s), clock: ck}
	defer h.srv.Close()
	phone := newSoftAuth(t, "platform")
	onb := h.signUp("erase@example.com")
	h.register(onb, uaIPhone, phone)
	loginID, _ := h.firstFactor(phone, uaIPhone)
	r := h.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": loginID}, "", uaIPhone)
	h.mustStatus(r, 200)
	tok := r.json(t)["access_token"].(string)

	req := httptest.NewRequest("GET", "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	acc, ok := s.Authenticate(req)
	if !ok {
		t.Fatal("not signed in")
	}
	ex, err := s.Export(context.Background(), acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	m := ex.(map[string]any)
	if len(m["passkeys"].([]map[string]any)) != 1 || m["signed_in_sessions"].(int) < 1 {
		t.Fatalf("export: %v", m)
	}
	rc, err := s.Erase(context.Background(), acc.ID)
	if err != nil || rc.Erased < 2 {
		t.Fatalf("erase: %+v %v", rc, err)
	}
	if _, ok := s.Authenticate(req); ok {
		t.Fatal("session still valid after erasure")
	}
	if docs, _ := st.All(context.Background(), "identity"); len(docs) != 0 {
		t.Fatalf("identity documents left: %d", len(docs))
	}
	if docs, _ := st.All(context.Background(), "session"); len(docs) != 0 {
		t.Fatalf("sessions left: %d", len(docs))
	}
	// The same contact can join again (its hash went with the account).
	h.signUp("erase@example.com")
}

func TestTrustedAddr(t *testing.T) {
	list := []string{"10.0.0.5", "172.16.0.0/12", "::1"}
	for ip, want := range map[string]bool{"10.0.0.5": true, "10.0.0.6": false, "172.20.3.4": true, "172.32.0.1": false,
		"::1": true, "::ffff:172.20.3.4": true, "not-an-ip": false} {
		if got := TrustedAddr(list, ip); got != want {
			t.Errorf("TrustedAddr(%q) = %v, want %v", ip, got, want)
		}
	}
}
