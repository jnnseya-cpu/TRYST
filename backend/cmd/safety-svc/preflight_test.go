package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/staffauth"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcauth"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcenv"
)

func TestPreflight(t *testing.T) {
	rk, _ := rsa.GenerateKey(rand.Reader, 2048)
	enc := base64.RawURLEncoding
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{
			{"kid": "k", "kty": "RSA", "n": enc.EncodeToString(rk.N.Bytes()), "e": enc.EncodeToString(big.NewInt(int64(rk.E)).Bytes())}}})
	}))
	defer jwks.Close()
	t.Setenv("INTERNAL_SECRET_SAFETY", strings.Repeat("aa", 32))
	t.Setenv("INTERNAL_SECRET_PLATFORM", strings.Repeat("bb", 32))
	platform := httptest.NewServer(svcenv.Verifier("prod", "safety", "INTERNAL_SECRET_PLATFORM").Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })))
	defer platform.Close()

	mk := func() *app {
		roster, _ := staffauth.ParseRoster("a@corp.example:lead,moderator;b@corp.example:moderator;c@corp.example:moderator")
		return &app{mode: "prod", st: store.NewMemory(),
			staff:      staffauth.Auth{Env: "prod", Roster: roster, Access: &staffauth.Access{Issuer: "https://t", Audience: "aud", JWKSURL: jwks.URL, HTTP: jwks.Client(), RequireAMR: "hwk"}},
			toPlatform: &svcauth.Client{BaseURL: platform.URL, Caller: "safety", Secret: svcenv.InternalSecret("prod", "INTERNAL_SECRET_PLATFORM")}}
	}
	failing := func(res []check) []string {
		var f []string
		for _, c := range res {
			if !c.OK {
				f = append(f, c.Name)
			}
		}
		return f
	}
	// The memory store fails the database check; everything else passes.
	res, ok := preflight(context.Background(), mk())
	if f := failing(res); ok || len(f) != 1 || f[0] != "safety database" {
		t.Fatalf("good setup failing: %v", f)
	}
	a := mk()
	delete(a.staff.Roster, "c@corp.example")
	if f := failing(must(preflight(context.Background(), a))); !contains(f, "staff roster") {
		t.Fatalf("two-person roster passed: %v", f)
	}
	a = mk()
	a.staff.Access.RequireAMR = ""
	if f := failing(must(preflight(context.Background(), a))); !contains(f, "hardware key") {
		t.Fatalf("unenforced hardware key passed: %v", f)
	}
	t.Setenv("INTERNAL_SECRET_PLATFORM", strings.Repeat("aa", 32))
	if f := failing(must(preflight(context.Background(), mk()))); !contains(f, "internal secrets") || !contains(f, "member platform") {
		t.Fatalf("same secret both ways passed: %v", f)
	}
	t.Setenv("INTERNAL_SECRET_PLATFORM", strings.Repeat("bb", 32))
	platform.Close()
	if f := failing(must(preflight(context.Background(), mk()))); !contains(f, "member platform") {
		t.Fatalf("unreachable platform passed: %v", f)
	}
	for _, c := range res {
		if strings.Contains(c.Detail, "@corp.example") || strings.Contains(c.Detail, "aaaa") {
			t.Fatalf("preflight output leaks: %s", c.Detail)
		}
	}
}

func must(r []check, _ bool) []check { return r }

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
