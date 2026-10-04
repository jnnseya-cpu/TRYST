package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	uaIPhone  = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) Mobile/15E148"
	uaDesktop = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/140.0"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

type harness struct {
	t     *testing.T
	srv   *httptest.Server
	clock *clock
}

type failingLiveness struct{}

func (failingLiveness) Verify(context.Context, string, string) (bool, error) { return false, nil }

func newHarness(t *testing.T, liveness LivenessProvider) *harness {
	t.Helper()
	ck := &clock{t: time.Now()}
	s, err := New(Config{
		RPID: "localhost", RPDisplayName: "Member services", RPOrigins: []string{"http://localhost:3000"},
		Env: "dev", ContactPepper: []byte("dev-pepper"), Liveness: liveness, Now: ck.now,
	}, NewStore())
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, srv: httptest.NewServer(s), clock: ck}
	t.Cleanup(h.srv.Close)
	return h
}

type resp struct {
	status int
	body   []byte
	header http.Header
}

func (r resp) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.body, &m); err != nil {
		t.Fatalf("not JSON (%d): %s", r.status, r.body)
	}
	return m
}

func (h *harness) do(method, path string, body any, token, ua string, extra ...string) resp {
	h.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case json.RawMessage:
		rd = bytes.NewReader(b)
	default:
		j, _ := json.Marshal(b)
		rd = bytes.NewReader(j)
	}
	req, _ := http.NewRequest(method, h.srv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	for i := 0; i+1 < len(extra); i += 2 {
		req.Header.Set(extra[i], extra[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return resp{res.StatusCode, b, res.Header}
}

func (h *harness) mustStatus(r resp, want int) {
	h.t.Helper()
	if r.status != want {
		h.t.Fatalf("status %d, want %d: %s", r.status, want, r.body)
	}
}

// signUp runs contact OTP sign-up and returns an onboarding token.
func (h *harness) signUp(contact string) string {
	r := h.do("POST", "/v1/auth/start", map[string]string{"contact": contact}, "", "")
	h.mustStatus(r, 202)
	otp := r.header.Get("X-Dev-OTP")
	if len(otp) != 6 {
		h.t.Fatalf("dev OTP header missing: %q", otp)
	}
	r = h.do("POST", "/v1/auth/verify", map[string]string{"contact": contact, "otp": otp}, "", "")
	h.mustStatus(r, 201)
	return r.json(h.t)["access_token"].(string)
}

func (h *harness) register(token, ua string, a *softAuth) map[string]any {
	r := h.do("POST", "/v1/auth/passkeys/register/begin", map[string]string{"attachment": a.attachment}, token, ua)
	h.mustStatus(r, 200)
	r = h.do("POST", "/v1/auth/passkeys/register/finish", a.create(r.body), token, ua)
	h.mustStatus(r, 201)
	return r.json(h.t)
}

// firstFactor completes B1/F1 and returns the login id and the response.
func (h *harness) firstFactor(a *softAuth, ua string) (string, map[string]any) {
	r := h.do("POST", "/v1/auth/login/begin", nil, "", ua)
	h.mustStatus(r, 200)
	var begin struct {
		LoginID string          `json:"login_id"`
		Options json.RawMessage `json:"options"`
	}
	_ = json.Unmarshal(r.body, &begin)
	cred, err := a.get(begin.Options, true)
	if err != nil {
		h.t.Fatal(err)
	}
	r = h.do("POST", "/v1/auth/login/passkey", map[string]any{"login_id": begin.LoginID, "credential": cred}, "", ua)
	h.mustStatus(r, 200)
	return begin.LoginID, r.json(h.t)
}

func (h *harness) me(token string) resp { return h.do("GET", "/v1/me", nil, token, "") }

// ---------------------------------------------------------------- tests

func TestMobileLoginNeedsTwoBiometricFactors(t *testing.T) {
	h := newHarness(t, DevLiveness{})
	phone := newSoftAuth(t, "platform")
	onb := h.signUp("alex@example.com")
	if got := h.register(onb, uaIPhone, phone); got["surface"] != "mobile_pwa" {
		t.Fatalf("surface %v", got["surface"])
	}

	loginID, f1 := h.firstFactor(phone, uaIPhone)
	if f1["next_factor"] != "B2" || f1["access_token"] != nil {
		t.Fatalf("after B1 there must be no session yet: %v", f1)
	}
	r := h.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": loginID}, "", uaIPhone)
	h.mustStatus(r, 200)
	tok := r.json(t)["access_token"].(string)
	me := h.me(tok)
	h.mustStatus(me, 200)
	m := me.json(t)
	if m["surface"] != "mobile_pwa" || m["session_kind"] != "full" {
		t.Fatalf("me: %v", m)
	}
}

func TestDesktopLoginNeedsPhoneApproval(t *testing.T) {
	h := newHarness(t, DevLiveness{})
	laptop := newSoftAuth(t, "platform")
	phone := newSoftAuth(t, "platform")
	onb := h.signUp("+447700900123")
	if got := h.register(onb, uaDesktop, laptop); got["surface"] != "desktop" {
		t.Fatalf("surface %v", got["surface"])
	}
	// Enrol the phone during onboarding with a one-time link.
	r := h.do("POST", "/v1/auth/enrol-device", nil, onb, uaDesktop)
	h.mustStatus(r, 201)
	enrol := r.json(t)["enrol_token"].(string)
	r = h.do("POST", "/v1/auth/enrol-device/redeem", map[string]string{"enrol_token": enrol}, "", uaIPhone)
	h.mustStatus(r, 200)
	h.register(r.json(t)["access_token"].(string), uaIPhone, phone)
	// The enrol link is single-use.
	h.mustStatus(h.do("POST", "/v1/auth/enrol-device/redeem", map[string]string{"enrol_token": enrol}, "", uaIPhone), 422)

	loginID, f1 := h.firstFactor(laptop, uaDesktop)
	if f1["next_factor"] != "F2" {
		t.Fatalf("desktop next factor %v", f1["next_factor"])
	}
	r = h.do("POST", "/v1/auth/login/cross-device", map[string]string{"login_id": loginID}, "", uaDesktop)
	h.mustStatus(r, 200)
	qr := r.json(t)["qr_token"].(string)

	poll := func(lid string) resp {
		return h.do("GET", "/v1/auth/login/cross-device/"+qr, nil, "", uaDesktop, "X-Login-Id", lid)
	}
	if p := poll(loginID); p.status != 200 || p.json(t)["state"] != "approval_pending" {
		t.Fatalf("pending: %d %s", p.status, p.body)
	}
	// Someone who only saw the QR code cannot collect the session.
	h.mustStatus(poll("not-the-login-id"), 404)

	r = h.do("POST", "/v1/auth/approvals/"+qr+"/begin", nil, "", uaIPhone)
	h.mustStatus(r, 200)
	var ab struct {
		Options json.RawMessage `json:"options"`
	}
	_ = json.Unmarshal(r.body, &ab)
	// The laptop's own passkey is not allowed to approve itself.
	if _, err := laptop.get(ab.Options, true); err != errNotAllowed {
		t.Fatal("approval must be restricted to the registered phone")
	}
	cred, err := phone.get(ab.Options, true)
	if err != nil {
		t.Fatal(err)
	}
	h.mustStatus(h.do("POST", "/v1/auth/approvals/"+qr, map[string]any{"decision": "approve", "credential": cred}, "", uaIPhone), 200)

	p := poll(loginID)
	h.mustStatus(p, 200)
	tok, _ := p.json(t)["access_token"].(string)
	if tok == "" {
		t.Fatalf("expected a session: %s", p.body)
	}
	if m := h.me(tok).json(t); m["surface"] != "desktop" || m["session_kind"] != "full" {
		t.Fatalf("me: %v", m)
	}
	h.mustStatus(poll(loginID), 404) // delivered once only
}

func TestDesktopKeyCannotStandInForPhoneApproval(t *testing.T) {
	h := newHarness(t, DevLiveness{})
	laptop := newSoftAuth(t, "platform")
	phone := newSoftAuth(t, "platform")
	onb := h.signUp("sam@example.com")
	h.register(onb, uaDesktop, laptop)
	r := h.do("POST", "/v1/auth/enrol-device", nil, onb, uaDesktop)
	r = h.do("POST", "/v1/auth/enrol-device/redeem", map[string]string{"enrol_token": r.json(t)["enrol_token"].(string)}, "", uaIPhone)
	h.register(r.json(t)["access_token"].(string), uaIPhone, phone)

	loginID, _ := h.firstFactor(laptop, uaDesktop)
	r = h.do("POST", "/v1/auth/login/cross-device", map[string]string{"login_id": loginID}, "", uaDesktop)
	qr := r.json(t)["qr_token"].(string)
	r = h.do("POST", "/v1/auth/approvals/"+qr+"/begin", nil, "", uaDesktop)
	var ab struct {
		Options json.RawMessage `json:"options"`
	}
	_ = json.Unmarshal(r.body, &ab)
	forged, _ := laptop.get(ab.Options, false) // ignore allowCredentials, like a malicious client
	r = h.do("POST", "/v1/auth/approvals/"+qr, map[string]any{"decision": "approve", "credential": forged}, "", uaDesktop)
	if r.status == 200 {
		t.Fatal("a desktop credential must never satisfy F2")
	}
	p := h.do("GET", "/v1/auth/login/cross-device/"+qr, nil, "", uaDesktop, "X-Login-Id", loginID)
	if p.status == 200 && p.json(t)["access_token"] != nil {
		t.Fatal("no session may be issued")
	}
}

func TestSecondSecurityKeyPath(t *testing.T) {
	h := newHarness(t, DevLiveness{})
	laptop := newSoftAuth(t, "platform")
	key := newSoftAuth(t, "cross-platform")
	onb := h.signUp("kim@example.com")
	h.register(onb, uaDesktop, laptop)
	if got := h.register(onb, uaDesktop, key); got["kind"] != KindRoamingKey {
		t.Fatalf("kind %v", got["kind"])
	}
	loginID, _ := h.firstFactor(laptop, uaDesktop)
	r := h.do("POST", "/v1/auth/login/second-key/begin", map[string]string{"login_id": loginID}, "", uaDesktop)
	h.mustStatus(r, 200)
	var sb struct {
		Options json.RawMessage `json:"options"`
	}
	_ = json.Unmarshal(r.body, &sb)
	cred, err := key.get(sb.Options, true)
	if err != nil {
		t.Fatal(err)
	}
	r = h.do("POST", "/v1/auth/login/second-key", map[string]any{"login_id": loginID, "credential": cred}, "", uaDesktop)
	h.mustStatus(r, 200)
	if r.json(t)["access_token"] == nil {
		t.Fatal("expected session")
	}

	// Signing in with the key first leaves no *other* key for F2.
	loginID, _ = h.firstFactor(key, uaDesktop)
	h.mustStatus(h.do("POST", "/v1/auth/login/second-key/begin", map[string]string{"login_id": loginID}, "", uaDesktop), 422)
}

func TestSameKeyCannotBeBothFactors(t *testing.T) {
	h := newHarness(t, DevLiveness{})
	k1 := newSoftAuth(t, "cross-platform")
	k2 := newSoftAuth(t, "cross-platform")
	onb := h.signUp("lee@example.com")
	h.register(onb, uaDesktop, k1)
	h.register(onb, uaDesktop, k2)
	loginID, _ := h.firstFactor(k1, uaDesktop)
	r := h.do("POST", "/v1/auth/login/second-key/begin", map[string]string{"login_id": loginID}, "", uaDesktop)
	var sb struct {
		Options json.RawMessage `json:"options"`
	}
	_ = json.Unmarshal(r.body, &sb)
	if _, err := k1.get(sb.Options, true); err != errNotAllowed {
		t.Fatal("the first key must be excluded from the second factor")
	}
	forged, _ := k1.get(sb.Options, false)
	r = h.do("POST", "/v1/auth/login/second-key", map[string]any{"login_id": loginID, "credential": forged}, "", uaDesktop)
	if r.status == 200 {
		t.Fatal("reusing the first key as F2 must fail")
	}
}

func TestOTPNeverAddsAPasskeyToAnExistingAccount(t *testing.T) {
	h := newHarness(t, DevLiveness{})
	onb := h.signUp("dana@example.com")
	h.register(onb, uaIPhone, newSoftAuth(t, "platform"))
	r := h.do("POST", "/v1/auth/start", map[string]string{"contact": "dana@example.com"}, "", "")
	h.mustStatus(r, 202)
	r = h.do("POST", "/v1/auth/verify", map[string]string{"contact": "dana@example.com", "otp": r.header.Get("X-Dev-OTP")}, "", "")
	h.mustStatus(r, 409)
	if !strings.Contains(string(r.body), "account_exists_sign_in") {
		t.Fatalf("%s", r.body)
	}
}

func TestOnboardingSessionCannotAddDevicesAfterSealing(t *testing.T) {
	h := newHarness(t, DevLiveness{})
	phone := newSoftAuth(t, "platform")
	onb := h.signUp("rae@example.com")
	h.register(onb, uaIPhone, phone)
	loginID, _ := h.firstFactor(phone, uaIPhone)
	h.mustStatus(h.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": loginID}, "", uaIPhone), 200)
	// The account is now sealed; the old onboarding token can no longer register passkeys.
	h.mustStatus(h.do("POST", "/v1/auth/passkeys/register/begin", nil, onb, uaIPhone), 403)
	h.mustStatus(h.do("POST", "/v1/auth/enrol-device", nil, onb, uaIPhone), 403)
}

func TestLivenessFailsClosedWithoutProvider(t *testing.T) {
	h := newHarness(t, NoLiveness{})
	phone := newSoftAuth(t, "platform")
	h.register(h.signUp("jo@example.com"), uaIPhone, phone)
	loginID, _ := h.firstFactor(phone, uaIPhone)
	r := h.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": loginID}, "", uaIPhone)
	h.mustStatus(r, 503)
	if r.json(t)["access_token"] != nil {
		t.Fatal("no session without B2")
	}
}

func TestLockoutAfterRepeatedFailures(t *testing.T) {
	h := newHarness(t, failingLiveness{})
	phone := newSoftAuth(t, "platform")
	h.register(h.signUp("max@example.com"), uaIPhone, phone)
	for i := 0; i < lockoutThreshold; i++ {
		loginID, _ := h.firstFactor(phone, uaIPhone)
		h.mustStatus(h.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": loginID}, "", uaIPhone), 401)
	}
	// Now locked: even a valid passkey gets the generic failure.
	r := h.do("POST", "/v1/auth/login/begin", nil, "", uaIPhone)
	var begin struct {
		LoginID string          `json:"login_id"`
		Options json.RawMessage `json:"options"`
	}
	_ = json.Unmarshal(r.body, &begin)
	cred, _ := phone.get(begin.Options, true)
	h.mustStatus(h.do("POST", "/v1/auth/login/passkey", map[string]any{"login_id": begin.LoginID, "credential": cred}, "", uaIPhone), 401)
	h.clock.add(lockoutDuration + time.Second)
	h.firstFactor(phone, uaIPhone) // unlocked again
}

func TestNoUserVerificationIsRejected(t *testing.T) {
	h := newHarness(t, DevLiveness{})
	a := newSoftAuth(t, "platform")
	a.userVerify = false
	onb := h.signUp("uv@example.com")
	r := h.do("POST", "/v1/auth/passkeys/register/begin", nil, onb, uaIPhone)
	r = h.do("POST", "/v1/auth/passkeys/register/finish", a.create(r.body), onb, uaIPhone)
	if r.status == 201 {
		t.Fatal("a passkey without user verification (biometric/PIN) must be refused")
	}
}

func TestDesktopSessionIdleExpiry(t *testing.T) {
	h := newHarness(t, DevLiveness{})
	k1, k2 := newSoftAuth(t, "platform"), newSoftAuth(t, "cross-platform")
	onb := h.signUp("idle@example.com")
	h.register(onb, uaDesktop, k1)
	h.register(onb, uaDesktop, k2)
	loginID, _ := h.firstFactor(k1, uaDesktop)
	r := h.do("POST", "/v1/auth/login/second-key/begin", map[string]string{"login_id": loginID}, "", uaDesktop)
	var sb struct {
		Options json.RawMessage `json:"options"`
	}
	_ = json.Unmarshal(r.body, &sb)
	cred, _ := k2.get(sb.Options, true)
	r = h.do("POST", "/v1/auth/login/second-key", map[string]any{"login_id": loginID, "credential": cred}, "", uaDesktop)
	tok := r.json(t)["access_token"].(string)
	h.mustStatus(h.me(tok), 200)
	h.clock.add(16 * time.Minute)
	h.mustStatus(h.me(tok), 401)
}

func TestBurnSignsOutEverywhere(t *testing.T) {
	h := newHarness(t, DevLiveness{})
	phone := newSoftAuth(t, "platform")
	h.register(h.signUp("burn@example.com"), uaIPhone, phone)
	var toks []string
	for i := 0; i < 2; i++ {
		loginID, _ := h.firstFactor(phone, uaIPhone)
		r := h.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": loginID}, "", uaIPhone)
		toks = append(toks, r.json(t)["access_token"].(string))
	}
	h.mustStatus(h.do("POST", "/v1/burn", nil, toks[0], uaIPhone), 204)
	for _, tk := range toks {
		h.mustStatus(h.me(tk), 401)
	}
}

func TestJurisdictionGateAtSignUp(t *testing.T) {
	h := newHarness(t, DevLiveness{})
	r := h.do("POST", "/v1/auth/start", map[string]string{"contact": "x@example.com"}, "", "")
	r = h.do("POST", "/v1/auth/verify", map[string]string{"contact": "x@example.com", "otp": r.header.Get("X-Dev-OTP"), "jurisdiction": "AE"}, "", "")
	h.mustStatus(r, 403)
}

func TestWrongOTPAndAttemptLimit(t *testing.T) {
	h := newHarness(t, DevLiveness{})
	r := h.do("POST", "/v1/auth/start", map[string]string{"contact": "otp@example.com"}, "", "")
	good := r.header.Get("X-Dev-OTP")
	for i := 0; i < otpMaxAttempts; i++ {
		h.mustStatus(h.do("POST", "/v1/auth/verify", map[string]string{"contact": "otp@example.com", "otp": "000000x"}, "", ""), 422)
	}
	h.mustStatus(h.do("POST", "/v1/auth/verify", map[string]string{"contact": "otp@example.com", "otp": good}, "", ""), 422)
}

func TestProdConfigGuards(t *testing.T) {
	if _, err := New(Config{RPID: "x.test", RPDisplayName: "x", RPOrigins: []string{"https://x.test"}, Env: "prod", ContactPepper: bytes.Repeat([]byte{1}, 32), Liveness: DevLiveness{}}, NewStore()); err == nil {
		t.Fatal("DevLiveness must be refused in prod")
	}
	if _, err := New(Config{RPID: "x.test", RPDisplayName: "x", RPOrigins: []string{"https://x.test"}, Env: "prod", ContactPepper: []byte("short")}, NewStore()); err == nil {
		t.Fatal("a short pepper must be refused in prod")
	}
	s, err := New(Config{RPID: "x.test", RPDisplayName: "x", RPOrigins: []string{"https://x.test"}, Env: "prod", ContactPepper: bytes.Repeat([]byte{1}, 32)}, NewStore())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s)
	defer srv.Close()
	res, _ := http.Post(srv.URL+"/v1/auth/start", "application/json", strings.NewReader(`{"contact":"p@example.com"}`))
	if res.Header.Get("X-Dev-OTP") != "" {
		t.Fatal("the OTP must never be exposed outside dev")
	}
}

func TestSecurityActivityIsRealAndPrivate(t *testing.T) {
	h := newHarness(t, failingLiveness{})
	phone := newSoftAuth(t, "platform")
	onb := h.signUp("sec@example.com")
	h.register(onb, uaIPhone, phone)
	// One failed second factor...
	loginID, _ := h.firstFactor(phone, uaIPhone)
	h.mustStatus(h.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": loginID}, "", uaIPhone), 401)
	// Onboarding sessions cannot read security activity.
	h.mustStatus(h.do("GET", "/v1/me/security", nil, onb, ""), 401)

	h2 := newHarness(t, DevLiveness{})
	p2 := newSoftAuth(t, "platform")
	h2.register(h2.signUp("sec2@example.com"), uaIPhone, p2)
	var tok string
	for i := 0; i < 2; i++ {
		lid, _ := h2.firstFactor(p2, uaIPhone)
		tok = h2.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": lid}, "", uaIPhone).json(t)["access_token"].(string)
	}
	r := h2.do("GET", "/v1/me/security?days=7", nil, tok, "")
	h2.mustStatus(r, 200)
	var sec struct {
		Days   int `json:"days"`
		Series []struct {
			Phone, Computer, Failed int
		} `json:"series"`
		ActiveSessions int            `json:"active_sessions"`
		Passkeys       map[string]int `json:"passkeys"`
	}
	_ = json.Unmarshal(r.body, &sec)
	last := sec.Series[len(sec.Series)-1]
	if sec.Days != 7 || len(sec.Series) != 7 || last.Phone != 2 || last.Computer != 0 || sec.ActiveSessions != 2 || sec.Passkeys[KindPlatformUV] != 1 {
		t.Fatalf("unexpected security summary: %s", r.body)
	}
	if strings.Contains(string(r.body), "example.com") {
		t.Fatal("security activity must not contain the contact")
	}
}
