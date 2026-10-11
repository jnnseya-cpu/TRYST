package staffauth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

var enc = base64.RawURLEncoding

func sign(t *testing.T, key any, kid string, claims map[string]any) string {
	alg := "RS256"
	if _, ok := key.(*ecdsa.PrivateKey); ok {
		alg = "ES256"
	}
	h, _ := json.Marshal(map[string]string{"alg": alg, "kid": kid, "typ": "JWT"})
	c, _ := json.Marshal(claims)
	in := enc.EncodeToString(h) + "." + enc.EncodeToString(c)
	d := sha256.Sum256([]byte(in))
	var sig []byte
	switch k := key.(type) {
	case *rsa.PrivateKey:
		sig, _ = rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, d[:])
	case *ecdsa.PrivateKey:
		r, s, _ := ecdsa.Sign(rand.Reader, k, d[:])
		sig = make([]byte, 64)
		r.FillBytes(sig[:32])
		s.FillBytes(sig[32:])
	}
	return in + "." + enc.EncodeToString(sig)
}

func TestAccessJWT(t *testing.T) {
	rk, _ := rsa.GenerateKey(rand.Reader, 2048)
	ek, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	var fetches atomic.Int32
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		eb, _ := ek.PublicKey.Bytes()
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{
			{"kid": "r1", "kty": "RSA", "alg": "RS256", "n": enc.EncodeToString(rk.N.Bytes()), "e": enc.EncodeToString(big.NewInt(int64(rk.E)).Bytes())},
			{"kid": "e1", "kty": "EC", "crv": "P-256", "x": enc.EncodeToString(eb[1:33]), "y": enc.EncodeToString(eb[33:])},
		}})
	}))
	defer jwks.Close()
	now := time.Unix(1_800_000_000, 0)
	acc := &Access{Issuer: "https://team.cloudflareaccess.com", Audience: "aud-tag", JWKSURL: jwks.URL, Now: func() time.Time { return now }}
	auth := Auth{Env: "prod", Access: acc, Roster: map[string][]string{"mod@corp.example": {"moderator"}}}
	if err := auth.Check(); err != nil {
		t.Fatal(err)
	}
	good := map[string]any{"iss": acc.Issuer, "aud": []string{"aud-tag"}, "exp": now.Add(time.Hour).Unix(), "nbf": now.Add(-time.Minute).Unix(), "email": "Mod@corp.example"}
	req := func(tok string) *http.Request {
		r := httptest.NewRequest("GET", "/ops/v1/cases", nil)
		r.Header.Set("Cf-Access-Jwt-Assertion", tok)
		return r
	}
	for _, k := range []struct {
		key any
		kid string
	}{{rk, "r1"}, {ek, "e1"}} {
		if s, ok := auth.Authenticate(req(sign(t, k.key, k.kid, good))); !ok || s.ID != "mod@corp.example" || !s.Has("moderator") {
			t.Fatalf("%s: %v %v", k.kid, s, ok)
		}
	}
	bad := func(name string, mut func(m map[string]any)) {
		c := map[string]any{}
		for k, v := range good {
			c[k] = v
		}
		mut(c)
		if _, ok := auth.Authenticate(req(sign(t, rk, "r1", c))); ok {
			t.Errorf("%s accepted", name)
		}
	}
	bad("expired", func(m map[string]any) { m["exp"] = now.Add(-time.Hour).Unix() })
	bad("wrong audience", func(m map[string]any) { m["aud"] = []string{"other"} })
	bad("wrong issuer", func(m map[string]any) { m["iss"] = "https://evil.example" })
	bad("not yet valid", func(m map[string]any) { m["nbf"] = now.Add(time.Hour).Unix() })
	bad("not on the roster", func(m map[string]any) { m["email"] = "someone@corp.example" })
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	if _, ok := auth.Authenticate(req(sign(t, other, "r1", good))); ok {
		t.Error("token signed by another key accepted")
	}
	tok := sign(t, rk, "r1", good)
	if _, ok := auth.Authenticate(req(tok[:len(tok)-4] + "AAAA")); ok {
		t.Error("tampered signature accepted")
	}
	// alg confusion: an RSA key with an ES256 header.
	if _, ok := auth.Authenticate(req(sign(t, ek, "r1", good))); ok {
		t.Error("algorithm confusion accepted")
	}
	before := fetches.Load()
	for i := 0; i < 20; i++ {
		auth.Authenticate(req(sign(t, rk, "unknown-kid", good)))
	}
	if fetches.Load()-before > 1 {
		t.Fatalf("unknown kids refetched the JWKS %d times", fetches.Load()-before)
	}
}

func TestHeadersRosterAndDevTokens(t *testing.T) {
	roster, err := ParseRoster("lead@corp.example:lead,moderator; mod@corp.example:moderator")
	if err != nil || len(roster) != 2 {
		t.Fatal(roster, err)
	}
	if _, err := ParseRoster("x@y:admin"); err == nil {
		t.Fatal("unknown role accepted")
	}
	a := Auth{Env: "prod", TrustedProxies: []string{"10.0.0.0/24"}, Roster: roster}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.7:5000"
	r.Header.Set("X-Staff-Identity", "lead@corp.example")
	r.Header.Set("X-Staff-Roles", "superuser")
	if s, ok := a.Authenticate(r); !ok || !s.Has("lead") || s.Has("superuser") {
		t.Fatalf("roles must come from the roster: %v %v", s, ok)
	}
	r.RemoteAddr = "203.0.113.1:5000"
	if _, ok := a.Authenticate(r); ok {
		t.Fatal("header believed from outside the proxy")
	}
	if err := (Auth{Env: "prod", TrustedProxies: []string{"10.0.0.1"}}).Check(); err == nil {
		t.Fatal("production without a roster accepted")
	}
	if err := (Auth{Env: "prod", DevTokens: map[string]Staff{"x": {}}}).Check(); err != ErrDevTokensInProd {
		t.Fatal("dev tokens accepted in prod")
	}
	d := Auth{Env: "dev", DevTokens: map[string]Staff{HashToken("tok-0123456789abcdef"): {ID: "mod1", Roles: []string{"moderator"}}}}
	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Staff tok-0123456789abcdef")
	if s, ok := d.Authenticate(r); !ok || s.ID != "mod1" {
		t.Fatal("dev token refused in dev")
	}
	d.Env = "prod"
	if _, ok := d.Authenticate(r); ok {
		t.Fatal("dev token accepted outside dev")
	}
}

func TestRequireHardwareKey(t *testing.T) {
	rk, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{
			{"kid": "r1", "kty": "RSA", "n": enc.EncodeToString(rk.N.Bytes()), "e": enc.EncodeToString(big.NewInt(int64(rk.E)).Bytes())}}})
	}))
	defer jwks.Close()
	now := time.Unix(1_800_000_000, 0)
	acc := &Access{Issuer: "https://t.example", Audience: "aud", JWKSURL: jwks.URL, RequireAMR: "hwk", Now: func() time.Time { return now }}
	claims := func(amr []string) map[string]any {
		return map[string]any{"iss": "https://t.example", "aud": "aud", "exp": now.Add(time.Hour).Unix(), "email": "mod@corp.example", "amr": amr}
	}
	if c, err := acc.Inspect(sign(t, rk, "r1", claims([]string{"pwd", "hwk"}))); err != nil || c.Email != "mod@corp.example" {
		t.Fatalf("hardware key sign-in refused: %v", err)
	}
	for _, amr := range [][]string{{"pwd", "otp"}, {"pwd"}, nil} {
		if _, err := acc.Inspect(sign(t, rk, "r1", claims(amr))); err != ErrAMR {
			t.Fatalf("amr %v accepted: %v", amr, err)
		}
	}
}
