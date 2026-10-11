package main

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/svcauth"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcenv"
)

func devEnv(t *testing.T) {
	for _, k := range []string{"DATABASE_URL_IDENTITY", "DATABASE_URL_PROFILE", "DATABASE_URL_KEYS", "DATABASE_URL_COMMERCE",
		"DATABASE_URL_LEDGER", "STRIPE_SECRET_KEY", "ARC_URL", "LIVENESS_URL", "INTERNAL_SECRET_SAFETY", "INTERNAL_SECRET_PLATFORM", "STAFF_ROSTER"} {
		t.Setenv(k, "")
	}
	t.Setenv("TRYST_ENV", "dev")
}

// TestSignalsReachTheSafetyServiceDurably: signals queue in the profile database and are
// delivered once the safety service answers; nothing is lost while it is down.
func TestSignalsReachTheSafetyServiceDurably(t *testing.T) {
	devEnv(t)
	f, _ := startFakeSafety(t)
	a := build()
	f.down = true
	a.prof.OnSignal("th_1", "acc-a", "acc-b", "child_safety")
	a.prof.OnSignal("th_1", "acc-a", "acc-b", "fraud")
	if left := a.signals.Deliver(context.Background()); left != 2 {
		t.Fatalf("left while down = %d", left)
	}
	f.mu.Lock()
	f.down = false
	f.mu.Unlock()
	if left := a.signals.Deliver(context.Background()); left != 0 {
		t.Fatalf("left = %d", left)
	}
	if len(f.signals) != 2 || f.signals[0]["kind"] != "child_safety" || f.signals[0]["subject"] != "acc-b" || f.signals[0]["id"] == "" {
		t.Fatalf("signals: %v", f.signals)
	}
	// Account deletion reaches the safety service's erasure.
	if _, _, err := a.rights.EraseAccount(context.Background(), "acc-b"); err != nil {
		t.Fatal(err)
	}
	if len(f.erased) != 1 || f.erased[0] != "acc-b" {
		t.Fatalf("erased: %v", f.erased)
	}
}

// TestPlatformInternalAPI: only the safety service's signature opens it, and it gives
// moderation what it needs and no more.
func TestPlatformInternalAPI(t *testing.T) {
	devEnv(t)
	startFakeSafety(t)
	a := build()
	srv := httptest.NewServer((&svcauth.Verifier{Secrets: map[string][]byte{"safety": svcenv.InternalSecret("dev", "INTERNAL_SECRET_PLATFORM")}}).Wrap(internalAPI(a.srv, a.prof, a.notifier)))
	defer srv.Close()
	c := &svcauth.Client{BaseURL: srv.URL, Caller: "safety", Secret: svcenv.InternalSecret("dev", "INTERNAL_SECRET_PLATFORM")}
	ctx := context.Background()
	if err := c.Do(ctx, "POST", "/internal/v1/authenticate", map[string]string{"authorization": "Bearer forged"}, nil); !svcauth.IsStatus(err, 401) {
		t.Fatalf("forged member token: %v", err)
	}
	if err := c.Do(ctx, "PUT", "/internal/v1/members/acc-x/standing", map[string]string{"standing": "admin"}, nil); !svcauth.IsStatus(err, 400) {
		t.Fatalf("unknown standing: %v", err)
	}
	if err := c.Do(ctx, "PUT", "/internal/v1/members/acc-x/standing", map[string]string{"standing": "permanent"}, nil); err != nil {
		t.Fatal(err)
	}
	ex, _ := a.prof.Export(ctx, "acc-x")
	if ex.(map[string]any)["standing"] != "permanent" {
		t.Fatalf("standing not applied: %v", ex)
	}
	if err := c.Do(ctx, "POST", "/internal/v1/notices", map[string]string{"account": "acc-x", "kind": "safety", "link": "/account"}, nil); err != nil {
		t.Fatal(err)
	}
	if n, _ := a.notifier.Export(ctx, "acc-x"); n == nil {
		t.Fatal("notice not recorded")
	}
	if err := c.Do(ctx, "GET", "/internal/v1/threads/th_none/counterpart?member=acc-x", nil, nil); !svcauth.IsStatus(err, 404) {
		t.Fatalf("counterpart of a stranger: %v", err)
	}
	// Anyone else, including a member-facing caller, is refused.
	bad := &svcauth.Client{BaseURL: srv.URL, Caller: "profile", Secret: svcenv.InternalSecret("dev", "INTERNAL_SECRET_SAFETY")}
	if err := bad.Do(ctx, "PUT", "/internal/v1/members/acc-x/standing", map[string]string{"standing": "good"}, nil); !svcauth.IsStatus(err, 401) {
		t.Fatalf("wrong caller: %v", err)
	}
}

func TestPlatformPreflight(t *testing.T) {
	devEnv(t)
	_, url := startFakeSafety(t)
	a := build()
	out, ok := preflightPlatform(context.Background(), a)
	if !ok || out["safety_service_reachable"] != true {
		t.Fatalf("preflight: %v", out)
	}
	a.toSafety.BaseURL = url + "/nowhere"
	if _, ok := preflightPlatform(context.Background(), a); ok {
		t.Fatal("unreachable safety service passed")
	}
}
