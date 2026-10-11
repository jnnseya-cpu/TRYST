package main

import (
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/svcenv"
	"go.yaml.in/yaml/v3"
)

var param = regexp.MustCompile(`\{[^}]+\}`)

// TestConsoleAndMemberRoutesRefuseUnauthenticatedCallers drives every console operation
// (ops.v1.yaml) and the safety service's member operations (tryst.v1.yaml) signed out,
// with a forged bearer token and with a forged staff token: never 2xx, never 5xx, and the
// locked-down API headers on every answer.
func TestConsoleAndMemberRoutesRefuseUnauthenticatedCallers(t *testing.T) {
	for _, k := range []string{"DATABASE_URL_SAFETY", "STAFF_ROSTER", "STAFF_ACCESS_ISSUER", "STAFF_TRUSTED_PROXIES"} {
		t.Setenv(k, "")
	}
	t.Setenv("TRYST_ENV", "dev")
	t.Setenv("STAFF_DEV_TOKENS", "mod1:moderator:real-dev-token-0001")
	t.Setenv("PLATFORM_INTERNAL_URL", "http://127.0.0.1:1") // unreachable: members can't authenticate
	h := svcenv.APIHeaders(routes(build()))

	ops := map[string]bool{}
	read := func(file, prefix string, keep func(string) bool) {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var c struct {
			Paths map[string]map[string]any `yaml:"paths"`
		}
		if err := yaml.Unmarshal(b, &c); err != nil {
			t.Fatal(err)
		}
		for p, m := range c.Paths {
			if !keep(p) {
				continue
			}
			for method := range m {
				if method == "get" || method == "post" || method == "put" || method == "delete" {
					ops[strings.ToUpper(method)+" "+prefix+p] = true
				}
			}
		}
	}
	read("../../../contracts/openapi/ops.v1.yaml", "/ops/v1", func(p string) bool { return p != "/vouchers" })
	read("../../../contracts/openapi/tryst.v1.yaml", "/v1", func(p string) bool {
		return strings.HasPrefix(p, "/reports") || p == "/appeals" || p == "/me/standing"
	})
	if len(ops) < 12 {
		t.Fatalf("only %d operations", len(ops))
	}
	for k := range ops {
		method, path, _ := strings.Cut(k, " ")
		path = param.ReplaceAllString(path, "cs_000000000000000000")
		for _, auth := range []string{"", "Bearer forged", "Staff forged-token-000000"} {
			r := httptest.NewRequest(method, path, strings.NewReader("{}"))
			if auth != "" {
				r.Header.Set("Authorization", auth)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code/100 == 2 || w.Code >= 500 {
				t.Errorf("%s (%q): %d", k, auth, w.Code)
			}
			if w.Header().Get("Cache-Control") != "no-store" && w.Code != 404 && w.Code != 405 {
				t.Errorf("%s: Cache-Control %q", k, w.Header().Get("Cache-Control"))
			}
			if !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") {
				t.Errorf("%s: no CSP", k)
			}
		}
	}
	// The real dev token works, so the refusals above aren't just a broken console.
	r := httptest.NewRequest("GET", "/ops/v1/cases", nil)
	r.Header.Set("Authorization", "Staff real-dev-token-0001")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("dev token: %d", w.Code)
	}
}

func TestProductionRefusesUnsafeConfig(t *testing.T) {
	// Exercised as a subprocess: build() exits on bad configuration.
	if os.Getenv("SAFETY_CONFIG_CASE") != "" {
		build()
		return
	}
	cases := map[string][]string{
		"no staff sign-in":    {"TRYST_ENV=prod", "DATABASE_URL_SAFETY=", "PLATFORM_INTERNAL_URL=http://x"},
		"dev tokens in prod":  {"TRYST_ENV=prod", "STAFF_DEV_TOKENS=a:moderator:0123456789abcdef", "STAFF_TRUSTED_PROXIES=10.0.0.1", "STAFF_ROSTER=a@b.c:moderator"},
		"no roster":           {"TRYST_ENV=prod", "STAFF_TRUSTED_PROXIES=10.0.0.1"},
		"no platform url":     {"TRYST_ENV=prod", "STAFF_TRUSTED_PROXIES=10.0.0.1", "STAFF_ROSTER=a@b.c:moderator"},
		"no internal secrets": {"TRYST_ENV=prod", "STAFF_TRUSTED_PROXIES=10.0.0.1", "STAFF_ROSTER=a@b.c:moderator", "PLATFORM_INTERNAL_URL=http://x"},
	}
	for name, env := range cases {
		cmd := testCommand(t, append(env, "SAFETY_CONFIG_CASE=1"))
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("%s: started anyway", name)
		}
		if !strings.Contains(string(out), "required") && !strings.Contains(string(out), "forbidden") && !strings.Contains(string(out), "needs") {
			t.Errorf("%s: unclear refusal: %s", name, out)
		}
	}
}
