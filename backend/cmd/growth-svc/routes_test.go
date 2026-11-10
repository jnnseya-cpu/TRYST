package main

import (
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/svcenv"
	"go.yaml.in/yaml/v3"
)

var param = regexp.MustCompile(`\{[^}]+\}`)

func growthOps(t *testing.T) map[string]bool {
	b, err := os.ReadFile("../../../shared/contracts/openapi/ops.v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	ops := map[string]bool{}
	for p, m := range c.Paths {
		if !strings.HasPrefix(p, "/growth/") {
			continue
		}
		for method := range m {
			if method == "get" || method == "post" {
				ops[strings.ToUpper(method)+" /ops/v1"+p] = true
			}
		}
	}
	return ops
}

func testApp(t *testing.T) *app {
	for _, k := range []string{"DATABASE_URL_GROWTH", "STAFF_ROSTER", "STAFF_ACCESS_ISSUER", "STAFF_TRUSTED_PROXIES", "SITE_URL", "APP_URL"} {
		t.Setenv(k, "")
	}
	t.Setenv("TRYST_ENV", "dev")
	t.Setenv("STAFF_DEV_TOKENS", "ed1:editor:real-dev-token-0001,mod1:moderator:moderator-token-0001")
	t.Setenv("PLATFORM_INTERNAL_URL", "http://127.0.0.1:1")
	return build()
}

// TestConsoleRoutesRefuseUnauthenticatedCallers drives every growth operation in ops.v1.yaml
// signed out, with a forged token, and as a moderator (no growth role): never 2xx or 5xx,
// always the locked-down API headers.
func TestConsoleRoutesRefuseUnauthenticatedCallers(t *testing.T) {
	a := testApp(t)
	h := svcenv.APIHeaders(routes(a))
	ops := growthOps(t)
	if len(ops) < 15 {
		t.Fatalf("only %d growth operations in the contract", len(ops))
	}
	for k := range ops {
		method, path, _ := strings.Cut(k, " ")
		path = param.ReplaceAllString(path, "ac_000000000000")
		for _, auth := range []string{"", "Bearer forged", "Staff forged-token-000000", "Staff moderator-token-0001"} {
			r := httptest.NewRequest(method, path, strings.NewReader("{}"))
			if auth != "" {
				r.Header.Set("Authorization", auth)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code/100 == 2 || w.Code >= 500 || w.Code == 404 && path != "/ops/v1/growth/briefs/effective" && !strings.Contains(path, "ac_") {
				t.Errorf("%s (%q): %d", k, auth, w.Code)
			}
			if !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") {
				t.Errorf("%s: no CSP", k)
			}
		}
	}
	r := httptest.NewRequest("GET", "/ops/v1/growth/overview", nil)
	r.Header.Set("Authorization", "Staff real-dev-token-0001")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("editor token: %d", w.Code)
	}
}

// TestContractMatchesRoutes: every contract operation is served, and nothing undocumented is.
func TestContractMatchesRoutes(t *testing.T) {
	src, err := os.ReadFile("../../internal/growth/handlers.go")
	if err != nil {
		t.Fatal(err)
	}
	served := map[string]bool{}
	for _, m := range regexp.MustCompile(`HandleFunc\("((?:GET|POST) /ops/v1/growth/[^"]+)"`).FindAllStringSubmatch(string(src), -1) {
		served[m[1]] = true
	}
	var missing, extra []string
	ops := growthOps(t)
	for k := range ops {
		if !served[k] {
			missing = append(missing, k)
		}
	}
	for k := range served {
		if !ops[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing)+len(extra) > 0 {
		t.Fatalf("contract and routes differ: missing %v, undocumented %v", missing, extra)
	}
}

// TestPublicListenerServesGuidePagesOnly: the address the load balancer exposes can't reach
// the staff API, even with a staff token.
func TestPublicListenerServesGuidePagesOnly(t *testing.T) {
	a := testApp(t)
	pub := publicRoutes(a)
	for path := range growthOps(t) {
		_, p, _ := strings.Cut(path, " ")
		r := httptest.NewRequest("GET", param.ReplaceAllString(p, "x"), nil)
		r.Header.Set("Authorization", "Staff real-dev-token-0001")
		w := httptest.NewRecorder()
		pub.ServeHTTP(w, r)
		if w.Code != 404 && w.Code != 405 {
			t.Errorf("public listener answered %s: %d", p, w.Code)
		}
	}
}

// TestProductionRefusesMissingConfig: in production, missing staff sign-in, public
// addresses, the platform address or the database stops the service at start.
func TestProductionRefusesMissingConfig(t *testing.T) {
	if os.Getenv("GROWTH_PROD_CHILD") == "1" {
		build()
		os.Exit(0)
	}
	base := []string{"GROWTH_PROD_CHILD=1", "TRYST_ENV=prod", "PATH=" + os.Getenv("PATH")}
	full := append(base, "STAFF_TRUSTED_PROXIES=10.0.0.1", "STAFF_ROSTER=a@b:editor", "SITE_URL=https://www.x",
		"APP_URL=https://x", "PLATFORM_INTERNAL_URL=http://api:8091",
		"INTERNAL_SECRET_GROWTH=00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	for want, env := range map[string][]string{
		"staff sign-in is required":         without(full, "STAFF_TRUSTED_PROXIES"),
		"SITE_URL and APP_URL are required": without(full, "SITE_URL"),
		"PLATFORM_INTERNAL_URL is required": without(full, "PLATFORM_INTERNAL_URL"),
		"INTERNAL_SECRET_GROWTH":            without(full, "INTERNAL_SECRET_GROWTH"),
		"DATABASE_URL_GROWTH is required":   full,
	} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestProductionRefusesMissingConfig$")
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), want) {
			t.Errorf("want exit with %q, got %v: %s", want, err, out)
		}
	}
}

func without(env []string, key string) []string {
	var out []string
	for _, e := range env {
		if !strings.HasPrefix(e, key+"=") {
			out = append(out, e)
		}
	}
	return out
}
