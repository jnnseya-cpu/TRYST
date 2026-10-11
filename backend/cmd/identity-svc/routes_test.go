package main

import (
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// publicRoutes may answer 2xx without a session. Everything else in the contracts must
// refuse a signed-out caller. Keep this list short and justified.
var publicRoutes = map[string]string{
	"GET /v1/deletion-proofs/key": "verification key for completion proofs; not personal data",
	"POST /v1/auth/login/begin":   "sign-in starts without a session (WebAuthn challenge only)",
	"POST /v1/auth/logout":        "uniform 204 whether or not a session exists",
	"POST /v1/burn":               "uniform 204 whether or not a session exists (nothing to reveal)",
}

type contract struct {
	Paths map[string]map[string]any `yaml:"paths"`
}

func loadContract(t *testing.T, file, prefix string) map[string]string {
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var c contract
	if err := yaml.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for p, ops := range c.Paths {
		for m := range ops {
			switch m {
			case "get", "post", "put", "patch", "delete":
				out[strings.ToUpper(m)+" "+prefix+p] = prefix + p
			}
		}
	}
	return out
}

var param = regexp.MustCompile(`\{[^}]+\}`)

func concrete(p string) string {
	return param.ReplaceAllStringFunc(p, func(s string) string {
		switch s {
		case "{id}":
			if strings.Contains(p, "/threads/") {
				return "th_000000000000000000000000"
			}
			if strings.Contains(p, "/deletion-proofs/") {
				return "del_000000000000000000000000"
			}
			return "x_0000000000"
		default:
			return "x_0000000000"
		}
	})
}

// TestEveryContractRouteRefusesSignedOutCallers drives the real route table from both
// OpenAPI contracts: no route may answer 2xx without a session (except the allowlist),
// none may fail with 5xx, and every answer carries the no-store and nosniff headers.
func TestEveryContractRouteRefusesSignedOutCallers(t *testing.T) {
	for _, k := range []string{"DATABASE_URL_IDENTITY", "DATABASE_URL_PROFILE", "DATABASE_URL_KEYS", "DATABASE_URL_SAFETY",
		"DATABASE_URL_COMMERCE", "DATABASE_URL_LEDGER", "STRIPE_SECRET_KEY", "ARC_URL", "LIVENESS_URL"} {
		t.Setenv(k, "")
	}
	t.Setenv("TRYST_ENV", "dev")
	startFakeSafety(t)
	t.Setenv("STAFF_DEV_TOKENS", "")
	t.Setenv("RATE_LIMIT_PER_MIN", "1000000") // a 429 must not hide a 2xx
	h := apiHeaders(routes(build()))

	all := loadContract(t, "../../../shared/contracts/openapi/tryst.v1.yaml", "/v1")
	for k, v := range loadContract(t, "../../../shared/contracts/openapi/ops.v1.yaml", "/ops/v1") {
		all[k] = v
	}
	if len(all) < 60 {
		t.Fatalf("only %d operations read from the contracts", len(all))
	}
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var open []string
	for _, k := range keys {
		method, path := strings.SplitN(k, " ", 2)[0], all[k]
		for _, auth := range []string{"", "Bearer not-a-real-token", "Staff not-a-real-token"} {
			req := httptest.NewRequest(method, concrete(path), strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			if auth != "" {
				req.Header.Set("Authorization", auth)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code >= 500 && w.Code != 503 {
				t.Errorf("%s (%q): %d server error", k, auth, w.Code)
			}
			if w.Code/100 == 2 {
				if _, ok := publicRoutes[k]; !ok {
					open = append(open, k)
				}
			}
			if w.Code != 404 && w.Code != 405 && w.Code/100 != 3 {
				if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
					t.Errorf("%s: X-Content-Type-Options = %q", k, got)
				}
				if got := w.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'none'") {
					t.Errorf("%s: Content-Security-Policy = %q", k, got)
				}
				if got := w.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
					t.Errorf("%s: Cache-Control = %q", k, got)
				}
			}
		}
	}
	if len(open) > 0 {
		t.Fatalf("routes answering 2xx without a session: %v", open)
	}
}
