package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

func TestEmergencyLockNeedsCodeAndOnlyRecoveryLifts(t *testing.T) {
	g := newAssuranceRig(t, store.NewMemory(), DevAssurance{})
	var mu sync.Mutex
	locks := map[string]bool{}
	g.srv.cfg.OnLock = func(a string, l bool) { mu.Lock(); locks[a] = l; mu.Unlock() }

	// Sign-up shows the code once.
	r := g.h.do("POST", "/v1/auth/start", map[string]string{"contact": "dee@example.com"}, "", "")
	r = g.h.do("POST", "/v1/auth/verify", map[string]string{"contact": "dee@example.com", "otp": r.header.Get("X-Dev-OTP")}, "", "")
	g.h.mustStatus(r, 201)
	m := r.json(t)
	code, onb := m["emergency_code"].(string), m["access_token"].(string)
	if len(code) != 14 {
		t.Fatalf("code: %q", code)
	}
	phone := newSoftAuth(t, "platform")
	g.h.register(onb, uaIPhone, phone)
	tok := g.login(phone)
	g.verify(tok, "1990-01", "E1234567")
	a, _ := g.account(tok)

	lock := func(contact, c string) int {
		return g.h.do("POST", "/v1/emergency-lock", map[string]string{"contact": contact, "code": c}, "", "").status
	}
	// Wrong code and unknown contacts get the same answer, and lock nothing.
	if lock("dee@example.com", "AAAA-AAAA-AAAA") != 202 || lock("nobody@example.com", code) != 202 {
		t.Fatal("non-uniform answer")
	}
	g.h.mustStatus(g.h.me(tok), 200)
	// The right code (any spacing or case) locks: sessions end, sign-in refused.
	if lock("DEE@example.com", " "+strings.ToLower(code)+" ") != 202 {
		t.Fatal("lock")
	}
	g.h.mustStatus(g.h.me(tok), 401)
	if _, ok := g.account(tok); ok || !locks[a.ID] {
		t.Fatal("not locked")
	}
	loginID, _ := g.h.firstFactor(phone, uaIPhone)
	if r := g.h.do("POST", "/v1/auth/login/liveness", map[string]string{"login_id": loginID}, "", uaIPhone); r.status != 403 || r.json(t)["code"] != "account_locked" {
		t.Fatalf("sign-in while locked: %d %s", r.status, r.body)
	}

	// Recovery lifts it and issues a new code.
	rid, _ := g.beginRecovery("dee@example.com")
	g.recoveryCheck(rid, "liveness", AssuranceResult{Result: "pass"})
	g.recoveryCheck(rid, "age", AssuranceResult{Result: "pass", IsAdult: true, BirthYM: "1990-01", DocumentNumber: "E1234567"})
	g.h.clock.add(RecoveryHold + time.Minute)
	r = g.h.do("POST", "/v1/auth/recovery/"+rid+"/finish", nil, "", "")
	g.h.mustStatus(r, 200)
	if nc := r.json(t)["emergency_code"]; nc == nil || nc == code || locks[a.ID] {
		t.Fatalf("after recovery: code %v, locked %v", nc, locks[a.ID])
	}
	// The used code is spent.
	if lock("dee@example.com", code) != 202 {
		t.Fatal("uniform")
	}
	fresh := newSoftAuth(t, "platform")
	g.h.register(r.json(t)["access_token"].(string), uaIPhone, fresh)
	tok = g.login(fresh)

	// A new code needs step-up.
	g.h.mustStatus(g.h.do("POST", "/v1/auth/emergency-code", nil, tok, uaIPhone), 403)

	// Attempts are limited per contact.
	for range emergencyMaxPerDay {
		lock("eve@example.com", "x")
	}
	if lock("eve@example.com", "x") != 429 {
		t.Fatal("no limit")
	}
}

func TestShieldKeysMatchDeviceHashes(t *testing.T) {
	g := newAssuranceRig(t, store.NewMemory(), DevAssurance{})
	tok := g.member("Fay@Example.com")
	a, _ := g.account(tok)
	d := sha256.Sum256([]byte("fay@example.com")) // what the partner's device sends
	if k := g.srv.BlindShield(hex.EncodeToString(d[:])); k == "" || k != g.srv.ShieldKeyOf(a.ID) {
		t.Fatal("device hash doesn't match the account's shield key")
	}
	if g.srv.BlindShield("not-a-digest") != "" {
		t.Fatal("junk accepted")
	}
}
