// Command growth-svc is TRYST's growth intelligence (D-73; docs/spec/10_Growth_Intelligence.md):
// it scores aggregate demand, issues Growth Briefs, drafts guide-page changes, applies them
// only after an editor approves, measures views and join clicks without cookies, and rolls
// back. It is invisible to members: no screen, notice, cookie or script in the member app.
//
// It holds no member data and has no credentials to any member database. The member
// platform answers it on one internal path with weekly counts by city above k = 50.
//
// Environment:
//
//	TRYST_ENV              dev | prod (default prod; prod refuses dev shortcuts)
//	PORT                   staff console API /ops/v1/growth/*, private network only (default 8083)
//	PUBLIC_ADDR            guide pages /guides/* only (default :8084; the load balancer's guides host)
//	DATABASE_URL_GROWTH    the growth database (required in prod)
//	PLATFORM_INTERNAL_URL  the member platform's internal API (required in prod)
//	INTERNAL_SECRET_GROWTH hex ≥ 32 bytes, shared with identity-svc
//	SITE_URL, APP_URL      public addresses for canonical links and the join button (required in prod)
//	GROWTH_INFERENCE_URL   optional: a model served inside the deployment (OpenAI-compatible)
//	GROWTH_INFERENCE_MODEL optional model name for that server
//	STAFF_*                the same staff sign-in and roster as safety-svc
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/growth"
	"github.com/jnnseya-cpu/tryst/backend/internal/metrics"
	"github.com/jnnseya-cpu/tryst/backend/internal/staffauth"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcauth"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcenv"
)

type app struct {
	mode            string
	svc             *growth.Service
	st              store.Store
	siteURL, appURL string
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		c := &http.Client{Timeout: 3 * time.Second}
		resp, err := c.Get("http://127.0.0.1:" + svcenv.Env("PORT", "8083") + "/readyz")
		if err != nil || resp.StatusCode != 200 {
			os.Exit(1)
		}
		os.Exit(0)
	}
	serve(build())
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
	siteURL, appURL := os.Getenv("SITE_URL"), os.Getenv("APP_URL")
	if siteURL == "" || appURL == "" {
		if mode != "dev" {
			log.Fatal("SITE_URL and APP_URL are required in production")
		}
		siteURL, appURL = "http://localhost:3002", "http://localhost:3000"
	}
	platformURL := os.Getenv("PLATFORM_INTERNAL_URL")
	if platformURL == "" {
		if mode != "dev" {
			log.Fatal("PLATFORM_INTERNAL_URL is required in production")
		}
		platformURL = "http://127.0.0.1:8091"
	}
	secret := svcenv.InternalSecret(mode, "INTERNAL_SECRET_GROWTH")
	st := svcenv.OpenStore(ctx, mode, "DATABASE_URL_GROWTH")
	svcenv.SingleInstance(ctx, st, "tryst-growth-svc") // state is in memory: one copy only
	svc, err := growth.New(ctx, st, growth.TrystProfile(), staff, nil)
	if err != nil {
		log.Fatal(err)
	}
	svc.Signals = growth.RemoteSignals{C: &svcauth.Client{BaseURL: platformURL, Caller: "growth",
		Secret: secret}}
	svc.Gen = &growth.Generator{DeploymentURL: os.Getenv("GROWTH_INFERENCE_URL"), Model: os.Getenv("GROWTH_INFERENCE_MODEL")}
	return &app{mode: mode, svc: svc, st: st, siteURL: siteURL, appURL: appURL}
}

// routes is the staff API (contract-tested from ops.v1.yaml).
func routes(a *app) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
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
	a.svc.Routes(mux)
	return mux
}

// publicRoutes serves the guide pages and nothing else.
func publicRoutes(a *app) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	a.svc.PublicRoutes(mux, a.siteURL, a.appURL)
	return mux
}

func serve(a *app) {
	go func() { // apply queued, approved changes; settle interrupted ones by receipt
		for range time.Tick(5 * time.Second) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			for _, r := range a.svc.Work(ctx) {
				log.Printf("growth worker: %s", r)
			}
			cancel()
		}
	}()
	go func() { // hourly: pull the platform's weekly counts and re-score
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			if _, err := a.svc.Refresh(ctx, "worker:schedule"); err != nil {
				log.Printf("growth refresh: %v", err)
			}
			cancel()
			time.Sleep(time.Hour)
		}
	}()
	pub := &http.Server{Addr: svcenv.Env("PUBLIC_ADDR", ":8084"), Handler: publicRoutes(a), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		if err := pub.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("public listener: %v", err)
		}
	}()
	hs := &http.Server{Addr: ":" + svcenv.Env("PORT", "8083"), Handler: metrics.Default.Middleware(svcenv.APIHeaders(routes(a))),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 120 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = pub.Shutdown(ctx)
		_ = hs.Shutdown(ctx)
	}()
	log.Printf("growth-svc (%s) staff API on %s, guide pages on %s", a.mode, hs.Addr, pub.Addr)
	if err := hs.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
