package notify

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

type members struct{}

func (members) Authenticate(r *http.Request) (identity.Account, bool) {
	id := r.Header.Get("X-Test-Member")
	return identity.Account{ID: id}, id != ""
}

// pushServer is a fake push service that checks VAPID like a real one.
type pushServer struct {
	t      *testing.T
	srv    *httptest.Server
	mu     sync.Mutex
	hits   int
	gone   bool
	bodies []int
}

func newPushServer(t *testing.T) *pushServer {
	p := &pushServer{t: t}
	p.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		p.mu.Lock()
		defer p.mu.Unlock()
		p.hits++
		p.bodies = append(p.bodies, len(b))
		if !p.verify(r) || r.Header.Get("TTL") == "" {
			w.WriteHeader(401)
			return
		}
		if p.gone {
			w.WriteHeader(410)
			return
		}
		w.WriteHeader(201)
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *pushServer) verify(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "vapid t=") {
		return false
	}
	parts := strings.SplitN(strings.TrimPrefix(h, "vapid t="), ", k=", 2)
	if len(parts) != 2 {
		return false
	}
	tok, k := parts[0], parts[1]
	kb, err := base64.RawURLEncoding.DecodeString(k)
	if err != nil {
		return false
	}
	pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), kb)
	if err != nil {
		return false
	}
	seg := strings.Split(tok, ".")
	if len(seg) != 3 {
		return false
	}
	sig, _ := base64.RawURLEncoding.DecodeString(seg[2])
	if len(sig) != 64 {
		return false
	}
	d := sha256.Sum256([]byte(seg[0] + "." + seg[1]))
	if !ecdsa.Verify(pub, d[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		return false
	}
	cb, _ := base64.RawURLEncoding.DecodeString(seg[1])
	var c struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	_ = json.Unmarshal(cb, &c)
	return c.Aud == "https://"+r.Host && c.Exp > time.Now().Unix() && strings.HasPrefix(c.Sub, "mailto:")
}

func (p *pushServer) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.hits
}

type rig struct {
	t   *testing.T
	svc *Service
	ps  *pushServer
	wp  *WebPush
	now time.Time
	st  store.Store
}

func newRig(t *testing.T) *rig {
	ps := newPushServer(t)
	key, _ := GenerateVAPIDKey()
	wp := &WebPush{Key: key, Subject: "mailto:ops@example.org", HTTP: ps.srv.Client(), AllowHost: HostAllowed([]string{"127.0.0.1"})}
	st := store.NewMemory()
	svc, err := New(members{}, st, wp)
	if err != nil {
		t.Fatal(err)
	}
	g := &rig{t: t, svc: svc, ps: ps, wp: wp, now: time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC), st: st}
	svc.now = func() time.Time { return g.now }
	return g
}

func (g *rig) do(method, path, who string, body any) (int, map[string]any) {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	if who != "" {
		r.Header.Set("X-Test-Member", who)
	}
	w := httptest.NewRecorder()
	g.svc.ServeHTTP(w, r)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestInboxIsMaskedAndPersistent(t *testing.T) {
	g := newRig(t)
	g.svc.Notify("ana", "message", "/messages/th_1")
	g.svc.Notify("ana", "match", "/messages")
	g.svc.Notify("ana", "nonsense", "")
	c, out := g.do("GET", "/v1/notices", "ana", nil)
	if c != 200 || out["unread"].(float64) != 2 {
		t.Fatalf("inbox: %d %v", c, out)
	}
	first := out["notices"].([]any)[0].(map[string]any)
	if first["kind"] != "match" || first["text"] != "You have a new match." {
		t.Fatalf("order/text: %v", first)
	}
	if c, _ := g.do("GET", "/v1/notices", "", nil); c != 401 {
		t.Fatalf("signed out: %d", c)
	}
	g.do("POST", "/v1/notices/read", "ana", nil)
	// A restart keeps the inbox.
	s2, _ := New(members{}, g.st, g.wp)
	g.svc = s2
	if _, out := g.do("GET", "/v1/notices", "ana", nil); out["unread"].(float64) != 0 || len(out["notices"].([]any)) != 2 {
		t.Fatalf("after restart: %v", out)
	}
	if g.ps.count() != 0 {
		t.Fatal("pushed while notifications were off (the default)")
	}
	for i := 0; i < maxNotices+10; i++ {
		g.svc.Notify("ana", "message", "")
	}
	if _, out := g.do("GET", "/v1/notices", "ana", nil); len(out["notices"].([]any)) != maxNotices {
		t.Fatal("inbox not capped")
	}
}

func TestPushIsContentFreeVapidSignedAndCoalesced(t *testing.T) {
	g := newRig(t)
	ep := g.ps.srv.URL + "/push/abc"
	if c, _ := g.do("PUT", "/v1/notifications/push", "ana", map[string]string{"endpoint": ep}); c != 409 {
		t.Fatalf("subscribe while off: %d", c)
	}
	if c, out := g.do("PUT", "/v1/notifications/settings", "ana", map[string]any{"push": true, "text": "Reminder"}); c != 200 || out["text"] != "Reminder" || out["vapid_public_key"] == "" {
		t.Fatalf("settings: %d %v", c, out)
	}
	for _, bad := range []string{"http://127.0.0.1/x", "https://evil.example/x", "https://169.254.169.254/latest", "https://user@127.0.0.1/x"} {
		if c, _ := g.do("PUT", "/v1/notifications/push", "ana", map[string]string{"endpoint": bad}); c != 422 {
			t.Fatalf("endpoint %s accepted: %d", bad, c)
		}
	}
	if c, out := g.do("PUT", "/v1/notifications/push", "ana", map[string]string{"endpoint": ep}); c != 200 || out["devices"].(float64) != 1 {
		t.Fatalf("subscribe: %d %v", c, out)
	}
	g.svc.Notify("ana", "message", "")
	g.svc.Wait()
	g.svc.Notify("ana", "message", "") // inside the coalescing window
	g.svc.Wait()
	if g.ps.count() != 1 {
		t.Fatalf("pushes = %d, want 1", g.ps.count())
	}
	g.ps.mu.Lock()
	if g.ps.bodies[0] != 0 {
		t.Fatal("push carried a payload")
	}
	g.ps.mu.Unlock()
	// A gone subscription is dropped.
	g.now = g.now.Add(3 * time.Minute)
	g.ps.mu.Lock()
	g.ps.gone = true
	g.ps.mu.Unlock()
	g.svc.Notify("ana", "message", "")
	g.svc.Wait()
	if _, out := g.do("GET", "/v1/notifications/settings", "ana", nil); out["devices"].(float64) != 0 {
		t.Fatalf("gone endpoint kept: %v", out)
	}
	// Turning push off forgets endpoints.
	g.do("PUT", "/v1/notifications/settings", "ana", map[string]any{"push": true})
	g.ps.mu.Lock()
	g.ps.gone = false
	g.ps.mu.Unlock()
	g.do("PUT", "/v1/notifications/push", "ana", map[string]string{"endpoint": ep})
	g.do("PUT", "/v1/notifications/settings", "ana", map[string]any{"push": false})
	if _, out := g.do("GET", "/v1/notifications/settings", "ana", nil); out["devices"].(float64) != 0 || out["push"] != false {
		t.Fatalf("push off: %v", out)
	}
}

func TestNeutralWording(t *testing.T) {
	g := newRig(t)
	for _, bad := range []string{"New TRYST match!", "see https://x.example", strings.Repeat("a", 41)} {
		if c, _ := g.do("PUT", "/v1/notifications/settings", "ana", map[string]any{"text": bad}); c != 422 {
			t.Fatalf("%q accepted: %d", bad, c)
		}
	}
	if _, out := g.do("PUT", "/v1/notifications/settings", "ana", map[string]any{"text": ""}); out["text"] != DefaultText {
		t.Fatalf("default text: %v", out)
	}
}

func TestExportAndErase(t *testing.T) {
	g := newRig(t)
	g.svc.Notify("ana", "renewal", "/billing")
	ex, _ := g.svc.Export(context.Background(), "ana")
	if len(ex.(map[string]any)["notices"].([]Notice)) != 1 {
		t.Fatalf("export: %v", ex)
	}
	if _, err := g.svc.Erase(context.Background(), "ana"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.st.Get(context.Background(), "inbox", "ana"); err == nil {
		t.Fatal("inbox left")
	}
}

func TestVAPIDKeyRoundTrip(t *testing.T) {
	k, _ := GenerateVAPIDKey()
	raw, _ := k.Bytes()
	k2, err := ParseVAPIDKey(base64.RawURLEncoding.EncodeToString(raw))
	if err != nil || !k2.PublicKey.Equal(&k.PublicKey) {
		t.Fatalf("round trip: %v", err)
	}
	if _, err := ParseVAPIDKey("short"); err == nil {
		t.Fatal("bad key accepted")
	}
}
