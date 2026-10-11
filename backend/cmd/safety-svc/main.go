// Command safety-svc is TRYST's standalone Trust & Safety service (D-59; 03 §7.3, §8;
// FR-116, FR-123): member reports, the case queue with deadlines from the escalation
// matrix, the moderator console API, enforcement with two-person permanent removal,
// independent appeals, mandatory external reports, and a hash-chained audit log. It owns
// SAFETY-DB and talks to the member platform only through signed internal APIs.
//
// Environment:
//
//	TRYST_ENV              dev | prod (default prod; prod refuses dev shortcuts)
//	PORT                   public API: /v1/reports*, /v1/appeals, /v1/me/standing, /ops/v1/* (default 8082)
//	INTERNAL_ADDR          internal API for the member platform (default 127.0.0.1:8092)
//	METRICS_ADDR           /metrics, internal only (default 127.0.0.1:9092)
//	DATABASE_URL_SAFETY    the safety database (required in prod)
//	DATABASE_URL_LEDGER    erasure ledger, read by "admin reapply-erasures" only
//	PLATFORM_INTERNAL_URL  the member platform's internal API (required in prod)
//	INTERNAL_SECRET_PLATFORM, INTERNAL_SECRET_SAFETY  hex ≥ 32 bytes, one per direction
//	STAFF_ACCESS_ISSUER, STAFF_ACCESS_AUDIENCE, STAFF_ACCESS_JWKS_URL  sign-in proxy token
//	STAFF_TRUSTED_PROXIES  alternative: header-asserting proxy addresses
//	STAFF_ROSTER           email:role[,role];... (required in prod)
//	STAFF_DEV_TOKENS       dev only: id:role:token,...
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/dsr"
	"github.com/jnnseya-cpu/tryst/backend/internal/metrics"
	"github.com/jnnseya-cpu/tryst/backend/internal/safetysvc"
	"github.com/jnnseya-cpu/tryst/backend/internal/staffauth"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcauth"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcenv"
)

type app struct {
	mode       string
	safety     *safetysvc.Service
	st         store.Store
	staff      staffauth.Auth
	toPlatform *svcauth.Client
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		c := &http.Client{Timeout: 3 * time.Second}
		resp, err := c.Get("http://127.0.0.1:" + svcenv.Env("PORT", "8082") + "/readyz")
		if err != nil || resp.StatusCode != 200 {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if len(os.Args) > 2 && os.Args[1] == "admin" {
		svcenv.Lock = svcenv.AdminLock(os.Args[2]) // read-only commands run beside the live service
	}
	a := build()
	if len(os.Args) > 1 && os.Args[1] == "admin" {
		os.Exit(runAdmin(a, os.Args[2:]))
	}
	serve(a)
}

func build() *app {
	mode := svcenv.Env("TRYST_ENV", "prod")
	ctx := context.Background()
	staff, err := staffauth.FromEnv(mode, os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	if mode != "dev" && staff.Access == nil && len(staff.TrustedProxies) == 0 {
		log.Fatal("staff sign-in is required in production: set STAFF_ACCESS_* (preferred) or STAFF_TRUSTED_PROXIES, and STAFF_ROSTER")
	}
	if mode == "dev" && len(staff.DevTokens) > 0 {
		log.Printf("DEV MODE: moderator console accepts dev staff tokens. Never use in production.")
	}
	platformURL := os.Getenv("PLATFORM_INTERNAL_URL")
	if platformURL == "" {
		if mode != "dev" {
			log.Fatal("PLATFORM_INTERNAL_URL is required in production")
		}
		platformURL = "http://127.0.0.1:8091"
	}
	toPlatform := &svcauth.Client{BaseURL: platformURL, Caller: "safety", Secret: svcenv.InternalSecret(mode, "INTERNAL_SECRET_PLATFORM")}
	st := svcenv.OpenStore(ctx, mode, "DATABASE_URL_SAFETY")
	svcenv.SingleInstance(ctx, st, "tryst-safety-svc") // state is in memory: one copy only
	svc, err := safetysvc.New(safetysvc.RemoteMembers{C: toPlatform}, staff, safetysvc.RemoteDirectory{C: toPlatform}, st)
	if err != nil {
		log.Fatal(err)
	}
	svc.Notices = safetysvc.RemoteNotices(toPlatform)
	return &app{mode: mode, safety: svc, st: st, staff: staff, toPlatform: toPlatform}
}

func serve(a *app) {
	svc := a.safety
	// Internal API for the member platform.
	internal := svcenv.Verifier(a.mode, "profile", "INTERNAL_SECRET_SAFETY")
	svcenv.ServeInternal(svcenv.Env("INTERNAL_ADDR", "127.0.0.1:8092"), internal.Wrap(svc.InternalHandler()))

	// Deliver queued standing changes and notices; retry what failed.
	go func() {
		for range time.Tick(15 * time.Second) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			svc.Flush(ctx)
			cancel()
		}
	}()

	m := metrics.Default
	m.Help("tryst_safety_cases_overdue_urgent", "Suspected-minor, image-abuse, sextortion and threat cases, and mandatory external reports, past their deadline.")
	m.Gauge("tryst_safety_cases_open", func() float64 { o, _, _ := svc.QueueStats(); return float64(o) })
	m.Gauge("tryst_safety_cases_overdue", func() float64 { _, od, _ := svc.QueueStats(); return float64(od) })
	m.Gauge("tryst_safety_cases_overdue_urgent", func() float64 { _, _, u := svc.QueueStats(); return float64(u) })
	m.Gauge("tryst_safety_external_reports_pending", func() float64 { return float64(svc.ExternalPending()) })
	m.Gauge("tryst_safety_outbox_pending", func() float64 { return float64(svc.OutboxPending(context.Background())) })
	go func() {
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", m.Handler())
		ms := &http.Server{Addr: svcenv.Env("METRICS_ADDR", "127.0.0.1:9092"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		if err := ms.ListenAndServe(); err != nil {
			log.Printf("metrics listener: %v", err)
		}
	}()

	hs := &http.Server{Addr: ":" + svcenv.Env("PORT", "8082"), Handler: m.Middleware(svcenv.APIHeaders(routes(a))), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 60 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	drained := make(chan struct{})
	go func() {
		<-stop
		log.Printf("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		_ = hs.Shutdown(ctx)
		svc.Flush(ctx) // last attempt; anything left stays queued in the database
		close(drained)
	}()
	log.Printf("safety-svc (%s) listening on %s", a.mode, hs.Addr)
	if err := hs.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	<-drained
}

// routes is the public surface; covered by routes_test.go from the contracts.
func routes(a *app) *http.ServeMux {
	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	root.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if p, ok := a.st.(interface{ Ping(context.Context) error }); ok {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := p.Ping(ctx); err != nil {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
		}
		_, _ = w.Write([]byte("ok"))
	})
	for _, p := range []string{"/v1/reports", "/v1/reports/", "/v1/appeals", "/v1/me/standing", "/ops/v1/"} {
		root.Handle(p, a.safety)
	}
	return root
}

// runAdmin: "verify" checks the database (the audit chain is verified on load) and
// "reapply-erasures" erases again every account in the erasure ledger after a restore.
func runAdmin(a *app, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: safety-svc admin preflight|check-token|verify|reapply-erasures")
		return 2
	}
	ctx := context.Background()
	var out any
	switch args[0] {
	case "verify":
		cases, err1 := a.st.All(ctx, "case")
		audit, err2 := a.st.All(ctx, "audit")
		if err1 != nil || err2 != nil {
			fmt.Fprintln(os.Stderr, "admin:", err1, err2)
			return 1
		}
		o, od, u := a.safety.QueueStats()
		out = map[string]any{"ok": true, "audit_chain": "verified on load", "records": map[string]int{"case": len(cases), "audit": len(audit)},
			"open": o, "overdue": od, "overdue_urgent": u, "outbox_pending": a.safety.OutboxPending(ctx)}
	case "preflight":
		res, ok := preflight(ctx, a)
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
		if !ok {
			return 1
		}
		return 0
	case "check-token":
		return checkToken(a)
	case "reapply-erasures":
		ledger := svcenv.OpenStore(ctx, a.mode, "DATABASE_URL_LEDGER")
		entries, err := ledger.All(ctx, "erasure")
		if err != nil {
			fmt.Fprintln(os.Stderr, "admin:", err)
			return 1
		}
		erased := map[string]bool{}
		for _, b := range entries {
			var e dsr.LedgerEntry
			if json.Unmarshal(b, &e) == nil {
				erased[e.Subject] = true
			}
		}
		n := 0
		accounts := a.safety.Accounts()
		for _, acc := range accounts {
			if erased[dsr.SubjectCode(acc)] {
				if _, err := a.safety.Erase(ctx, acc); err != nil {
					fmt.Fprintln(os.Stderr, "admin:", err)
					return 1
				}
				n++
			}
		}
		out = map[string]any{"ledger_entries": len(erased), "accounts_checked": len(accounts), "erased_again": n}
	default:
		fmt.Fprintln(os.Stderr, "unknown command", args[0])
		return 2
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
	return 0
}
