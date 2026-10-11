// Command identity-svc runs the TRYST identity service: sign-up, passkey registration and
// account-holder-only two-factor login (docs/spec/02_Shared_Contracts.md §3.3). It also
// mounts the profile and photo API (/v1/profile*) behind the same sign-in; in production
// that API is the profile module of core-svc.
//
// Environment:
//
//	TRYST_ENV       dev | prod (default prod; prod refuses dev shortcuts)
//	RP_ID           WebAuthn relying-party ID (e.g. localhost, app.example.com)
//	RP_ORIGINS      comma-separated allowed origins (e.g. http://localhost:3000)
//	CONTACT_PEPPER  hex, >= 32 bytes in prod (HSM-held in production)
//	PORT            default 8081
//	TRUSTED_PROXIES comma-separated proxy addresses whose X-Forwarded-For is believed
//	                (the web app's proxy, the edge gateway); dev default 127.0.0.1,::1
//	RATE_LIMIT_PER_MIN requests per client address per minute (default 120)
//	DATABASE_URL_IDENTITY, DATABASE_URL_PROFILE, DATABASE_URL_KEYS
//	                PostgreSQL DSNs, one database per domain (required in prod; dev falls
//	                back to memory when unset)
//	DATA_KEK        hex, 32 bytes: key-encryption key for member data keys (KMS in
//	                production; required whenever DATABASE_URL_KEYS is set)
//	DATABASE_URL_SAFETY  the Trust & Safety database (cases, decisions, audit chain)
//	STAFF_TRUSTED_PROXIES comma-separated addresses of the staff sign-in proxy whose
//	                X-Staff-Identity / X-Staff-Roles headers are believed (/ops/v1/*)
//	STAFF_DEV_TOKENS dev only: id:role:token,... for the moderator console
//	DATABASE_URL_COMMERCE  entitlements, Keys ledger, processed events, vouchers
//	STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET  payments (dev uses a local test checkout)
//	STRIPE_PRICE_<KEY>  Stripe Price id per catalogue key, e.g. STRIPE_PRICE_TRYST_PLUS
//	DSR_SIGNING_KEY hex, 32-byte Ed25519 seed for deletion completion proofs (KMS in
//	                production; required outside dev)
//	VAPID_PRIVATE_KEY base64url P-256 scalar for content-free Web Push; VAPID_SUBJECT
//	                mailto: or https contact for push services (push is off without a key
//	                outside dev)
//	POSTMARK_SERVER_TOKEN, OTP_EMAIL_FROM  sign-up codes by email (neutral sender name)
//	TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, TWILIO_MESSAGING_SERVICE (or TWILIO_FROM)
//	                sign-up codes by SMS. Production needs at least one channel.
//	LIVENESS_URL, LIVENESS_API_KEY, LIVENESS_SECRET  identity provider gateway for the B2
//	                login face check and photo face match (fail closed without it)
//	SIGHTENGINE_USER, SIGHTENGINE_SECRET, PHOTODNA_KEY  photo checks; without all of them
//	                (and LIVENESS_URL) every photo waits for a person
//	ARC_URL, ARC_API_KEY, ARC_SECRET  the alarm-receiving centre (safety line, Panic);
//	                required in production (D-48)
//	METRICS_ADDR    internal listener for /metrics (default 127.0.0.1:9090); never public
//	PUBLIC_URL      member site origin for checkout return links (default first RP origin)
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/commercesvc"
	"github.com/jnnseya-cpu/tryst/backend/internal/dsr"
	"github.com/jnnseya-cpu/tryst/backend/internal/gate"
	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/metrics"
	"github.com/jnnseya-cpu/tryst/backend/internal/notify"
	"github.com/jnnseya-cpu/tryst/backend/internal/outbound"
	"github.com/jnnseya-cpu/tryst/backend/internal/payments"
	"github.com/jnnseya-cpu/tryst/backend/internal/profilesvc"
	"github.com/jnnseya-cpu/tryst/backend/internal/safetysvc"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
	"github.com/jnnseya-cpu/tryst/backend/internal/vendors"
)

type logSender struct{}

// Send logs that a code was issued without logging the code or the contact (dev only; the
// code is returned in X-Dev-OTP in dev).
func (logSender) Send(string, string) error {
	log.Printf("otp issued (dev sender)")
	return nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// openStore connects to the database named by envVar. Production refuses to run without
// it; development falls back to memory (data is lost on restart) and says so.
func openStore(ctx context.Context, mode, envVar string) store.Store {
	dsn := os.Getenv(envVar)
	if dsn == "" {
		if mode != "dev" {
			log.Fatalf("%s is required in production", envVar)
		}
		log.Printf("DEV MODE: %s not set; using memory (lost on restart)", envVar)
		return store.NewMemory()
	}
	p, err := store.OpenPostgres(ctx, dsn)
	if err != nil {
		log.Fatalf("%s: %v", envVar, err)
	}
	return p
}

func openVault(mode string, keys store.Store) *store.Vault {
	kek, err := hex.DecodeString(os.Getenv("DATA_KEK"))
	if err != nil || len(kek) != 32 {
		if mode != "dev" {
			log.Fatal("DATA_KEK (hex, 32 bytes) is required in production")
		}
		kek = make([]byte, 32)
		_, _ = rand.Read(kek)
		log.Printf("DEV MODE: DATA_KEK not set; using a random key (stored photos unreadable after restart)")
	}
	v, err := store.NewVault(keys, kek)
	if err != nil {
		log.Fatal(err)
	}
	return v
}

// app is the wired service set, shared by the server and the admin commands.
type app struct {
	mode     string
	srv      *identity.Server
	prof     *profilesvc.Service
	safety   *safetysvc.Service
	commerce *commercesvc.Service
	notifier *notify.Service
	rights   *dsr.Service
	stores   map[string]store.Store // by database name, for readiness and admin
	vault    *store.Vault
}

func main() {
	// Container health check (the runtime image has no shell or curl).
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		c := &http.Client{Timeout: 3 * time.Second}
		resp, err := c.Get("http://127.0.0.1:" + env("PORT", "8081") + "/readyz")
		if err != nil || resp.StatusCode != 200 {
			os.Exit(1)
		}
		os.Exit(0)
	}
	a := build()
	if len(os.Args) > 1 && os.Args[1] == "admin" {
		os.Exit(runAdmin(a, os.Args[2:]))
	}
	serve(a)
}

// build reads the environment and wires every service. It starts nothing.
func build() *app {
	mode := env("TRYST_ENV", "prod")
	cfg := identity.Config{
		RPID:          env("RP_ID", "localhost"),
		RPDisplayName: "Member services", // neutral: shown in the device's passkey manager
		RPOrigins:     strings.Split(env("RP_ORIGINS", "http://localhost:3000"), ","),
		Env:           mode,
	}
	if v := os.Getenv("TRUSTED_PROXIES"); v != "" {
		cfg.TrustedProxies = strings.Split(v, ",")
	} else if mode == "dev" {
		cfg.TrustedProxies = []string{"127.0.0.1", "::1"}
	}
	if v := os.Getenv("RATE_LIMIT_PER_MIN"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			log.Fatal("RATE_LIMIT_PER_MIN must be a positive number")
		}
		cfg.RateLimitPerMin = n
	}
	if p := os.Getenv("CONTACT_PEPPER"); p != "" {
		b, err := hex.DecodeString(p)
		if err != nil {
			log.Fatal("CONTACT_PEPPER must be hex")
		}
		cfg.ContactPepper = b
	}
	if mode == "dev" {
		cfg.Liveness = identity.DevLiveness{}
		cfg.DevBirthYM = env("DEV_BIRTH_YM", "1988-04") // stands in for the ID check's birth year-month
		cfg.Sender = logSender{}
		if cfg.ContactPepper == nil {
			cfg.ContactPepper = []byte("dev-only-pepper")
		}
		log.Printf("DEV MODE: liveness auto-approves and OTPs are returned in X-Dev-OTP. Never use in production.")
	}
	out := otpSender()
	if out != nil {
		cfg.Sender = out
	} else if mode != "dev" {
		log.Fatal("sign-up codes can't be delivered: set POSTMARK_SERVER_TOKEN and/or TWILIO_* (see the header of this file)")
	}
	idGateway := gateway("LIVENESS")
	if idGateway != nil {
		cfg.Liveness = &vendors.Liveness{G: idGateway}
	}
	ctx := context.Background()
	idStore := openStore(ctx, mode, "DATABASE_URL_IDENTITY")
	profStore := openStore(ctx, mode, "DATABASE_URL_PROFILE")
	keyStore := openStore(ctx, mode, "DATABASE_URL_KEYS")
	vault := openVault(mode, keyStore)
	cfg.Persist = idStore
	// Notifications are created after identity (they authenticate through it); producers
	// reach them through this indirection.
	var notifier *notify.Service
	notifyFn := func(account, kind, link string) {
		if notifier != nil {
			notifier.Notify(account, kind, link)
		}
	}
	cfg.Notify = notifyFn
	srv, err := identity.New(cfg, identity.NewStore())
	if err != nil {
		log.Fatal(err)
	}
	var mod profilesvc.Moderator = profilesvc.NoModerator{}
	if u, sec, pd := os.Getenv("SIGHTENGINE_USER"), os.Getenv("SIGHTENGINE_SECRET"), os.Getenv("PHOTODNA_KEY"); u != "" && sec != "" && pd != "" && idGateway != nil {
		mod = &vendors.PhotoModerator{Sightengine: &vendors.Sightengine{User: u, Secret: sec}, PhotoDNA: &vendors.PhotoDNA{Key: pd}, Faces: idGateway}
	} else if mode == "dev" {
		mod = profilesvc.DevModerator{}
		log.Printf("DEV MODE: photo moderation vendors are simulated; AI-label checks run for real.")
	}
	prof, err := profilesvc.NewWithStore(srv, mod, mode, profStore, vault)
	if err != nil {
		log.Fatal(err)
	}
	prof.Notify = notifyFn
	notifier = openNotify(mode, srv, profStore)
	if arc := gateway("ARC"); arc != nil {
		if out == nil {
			log.Fatal("the safety line needs a message channel for trusted contacts (POSTMARK_* or TWILIO_*)")
		}
		prof.Alerts = &vendors.ARCAlerting{ARC: arc, Contact: out}
	} else if mode != "dev" {
		log.Fatal("ARC_URL is required in production: meet safety can't run without the safety line (D-48)")
	} else {
		prof.Alerts = &profilesvc.LogAlerting{}
		log.Printf("DEV MODE: safety alerts go to the log, not to real contacts or the alarm-receiving centre.")
	}
	staff := safetysvc.StaffAuth{Env: mode, DevTokens: map[string]safetysvc.Staff{}}
	if v := os.Getenv("STAFF_TRUSTED_PROXIES"); v != "" {
		staff.TrustedProxies = strings.Split(v, ",")
	}
	if v := os.Getenv("STAFF_DEV_TOKENS"); v != "" {
		for _, item := range strings.Split(v, ",") {
			parts := strings.SplitN(item, ":", 3)
			if len(parts) != 3 || len(parts[2]) < 16 {
				log.Fatal("STAFF_DEV_TOKENS entries are id:role:token with a token of 16+ characters")
			}
			h := sha256.Sum256([]byte(parts[2]))
			staff.DevTokens[hex.EncodeToString(h[:])] = safetysvc.Staff{ID: parts[0], Roles: []string{parts[1]}}
		}
	}
	safetyStore := openStore(ctx, mode, "DATABASE_URL_SAFETY")
	safety, err := safetysvc.New(srv, staff, prof, safetyStore)
	if err != nil {
		log.Fatal(err)
	}
	prof.OnSignal = safety.FromSignal
	safety.Notify = notifyFn

	commerceStore := openStore(ctx, mode, "DATABASE_URL_COMMERCE")
	commerce := openCommerce(mode, srv, staff, cfg.RPOrigins[0], notifyFn, commerceStore)

	// Data rights: export and erasure. Order matters: the profile part destroys the member's
	// data key and removes them from discovery first; identity goes last.
	seed, err := hex.DecodeString(os.Getenv("DSR_SIGNING_KEY"))
	if err != nil || len(seed) != 32 {
		if mode != "dev" {
			log.Fatal("DSR_SIGNING_KEY (hex, 32 bytes) is required in production")
		}
		seed = make([]byte, 32)
		_, _ = rand.Read(seed)
		log.Printf("DEV MODE: DSR_SIGNING_KEY not set; deletion proofs use a temporary key")
	}
	rights, err := dsr.New(func(r *http.Request) (string, bool) {
		a, ok := srv.Authenticate(r)
		return a.ID, ok
	}, idStore, seed, prof, safety, commerce, notifier, srv)
	if err != nil {
		log.Fatal(err)
	}
	// The erasure ledger lives in its own database with its own backups: after any restore,
	// erasures newer than the backup are applied again (admin reapply-erasures, D-57).
	ledger := openStore(ctx, mode, "DATABASE_URL_LEDGER")
	rights.Ledger = ledger
	return &app{mode: mode, srv: srv, prof: prof, safety: safety, commerce: commerce, notifier: notifier, rights: rights, vault: vault,
		stores: map[string]store.Store{"identity": idStore, "profile": profStore, "keys": keyStore, "safety": safetyStore, "commerce": commerceStore, "ledger": ledger}}
}

// serve runs the HTTP server, background sweeps and the metrics listener.
func serve(a *app) {
	mode, prof, safety, notifier, rights := a.mode, a.prof, a.safety, a.notifier, a.rights
	go func() {
		for range time.Tick(time.Minute) {
			rights.Sweep(context.Background())
		}
	}()
	go func() {
		for range time.Tick(30 * time.Second) {
			prof.SweepSafety(context.Background())
		}
	}()
	root := routes(a)

	m := metrics.Default
	m.Help("tryst_http_requests_total", "Requests by route shape, method and status.")
	m.Help("tryst_safety_cases_overdue_urgent", "Open suspected-minor, image-abuse, sextortion and threat cases past their deadline.")
	m.Help("tryst_safety_alert_failures_total", "Safety alerts that could not be delivered after a retry.")
	m.Help("tryst_erasure_seconds", "Account erasure duration (NFR-01: < 60 s p99).")
	m.Gauge("tryst_safety_cases_open", func() float64 { o, _, _ := safety.QueueStats(); return float64(o) })
	m.Gauge("tryst_safety_cases_overdue", func() float64 { _, od, _ := safety.QueueStats(); return float64(od) })
	m.Gauge("tryst_safety_cases_overdue_urgent", func() float64 { _, _, u := safety.QueueStats(); return float64(u) })
	go func() {
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", m.Handler())
		ms := &http.Server{Addr: env("METRICS_ADDR", "127.0.0.1:9090"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		if err := ms.ListenAndServe(); err != nil {
			log.Printf("metrics listener: %v", err)
		}
	}()

	addr := ":" + env("PORT", "8081")
	log.Printf("identity-svc (%s) listening on %s", mode, addr)
	hs := &http.Server{Addr: addr, Handler: m.Middleware(apiHeaders(root)), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 60 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second}

	// Graceful shutdown (container stop, deploys): stop taking requests, finish in-flight
	// ones, and let dispatched safety alerts and pushes complete.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	drained := make(chan struct{})
	go func() {
		<-stop
		log.Printf("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		_ = hs.Shutdown(ctx) // returns when in-flight requests are done
		prof.WaitAlerts()
		notifier.Wait()
		close(drained)
	}()
	if err := hs.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	<-drained
}

// openCommerce builds the commerce service: Stripe when configured, the local test checkout
// in dev, and no processor otherwise (purchases answer 503 until Stripe is approved).
func openCommerce(mode string, members *identity.Server, staff safetysvc.StaffAuth, origin string, notifyFn func(string, string, string), st store.Store) *commercesvc.Service {
	cfg := commercesvc.Config{Members: members, Store: st, Env: mode, MinTier: gate.V1, Notify: notifyFn,
		StaffLead: func(r *http.Request) (string, bool) {
			st, ok := staff.Authenticate(r)
			if !ok {
				return "", false
			}
			for _, role := range st.Roles {
				if role == "lead" {
					return st.ID, true
				}
			}
			return "", false
		}}
	var stripe *payments.Stripe
	if key := os.Getenv("STRIPE_SECRET_KEY"); key != "" {
		public := env("PUBLIC_URL", origin)
		stripe = &payments.Stripe{SecretKey: key, WebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"), Prices: map[string]payments.StripePrice{},
			SuccessURL: public + "/billing?done=1", CancelURL: public + "/billing"}
		if stripe.WebhookSecret == "" {
			log.Fatal("STRIPE_WEBHOOK_SECRET is required with STRIPE_SECRET_KEY")
		}
		for _, it := range commercesvc.Catalogue {
			if id := os.Getenv("STRIPE_PRICE_" + strings.ToUpper(it.Key)); id != "" {
				stripe.Prices[it.Key] = payments.StripePrice{PriceID: id, Subscription: it.Period != ""}
			}
		}
		cfg.Provider = stripe
	} else if mode == "dev" {
		cfg.Provider = commercesvc.NewDevProvider()
		cfg.MinTier = gate.V0
		log.Printf("DEV MODE: payments use a local test checkout; no money moves.")
	} else {
		log.Printf("STRIPE_SECRET_KEY not set: purchases are unavailable (503) until payments are configured.")
	}
	svc, err := commercesvc.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if stripe != nil {
		stripe.SubscriptionFor = svc.SubscriptionFor
	}
	return svc
}

// openNotify builds the notification service. Push needs a VAPID key; dev generates one.
func openNotify(mode string, members *identity.Server, st store.Store) *notify.Service {
	var push notify.Pusher
	var key *ecdsa.PrivateKey
	var err error
	if v := os.Getenv("VAPID_PRIVATE_KEY"); v != "" {
		if key, err = notify.ParseVAPIDKey(v); err != nil {
			log.Fatal(err)
		}
	} else if mode == "dev" {
		key, _ = notify.GenerateVAPIDKey()
		log.Printf("DEV MODE: VAPID_PRIVATE_KEY not set; push subscriptions won't survive a restart")
	} else {
		log.Printf("VAPID_PRIVATE_KEY not set: push notifications are unavailable (in-app notices still work)")
	}
	if key != nil {
		push = &notify.WebPush{Key: key, Subject: env("VAPID_SUBJECT", "mailto:privacy@example.invalid"), AllowHost: notify.HostAllowed(notify.DefaultPushHosts)}
	}
	n, err := notify.New(members, st, push)
	if err != nil {
		log.Fatal(err)
	}
	return n
}

// otpSender returns the configured email/SMS sender, or nil if none is configured.
func otpSender() *outbound.Sender {
	var out outbound.Sender
	if t := os.Getenv("POSTMARK_SERVER_TOKEN"); t != "" {
		out.Email = &outbound.Postmark{Token: t, From: env("OTP_EMAIL_FROM", "Member services <codes@example.invalid>")}
	}
	if sid := os.Getenv("TWILIO_ACCOUNT_SID"); sid != "" {
		out.SMS = &outbound.Twilio{AccountSID: sid, AuthToken: os.Getenv("TWILIO_AUTH_TOKEN"),
			MessagingService: os.Getenv("TWILIO_MESSAGING_SERVICE"), From: os.Getenv("TWILIO_FROM")}
		if out.SMS.AuthToken == "" || (out.SMS.MessagingService == "" && out.SMS.From == "") {
			log.Fatal("TWILIO_ACCOUNT_SID needs TWILIO_AUTH_TOKEN and TWILIO_MESSAGING_SERVICE or TWILIO_FROM")
		}
	}
	if out.Email == nil && out.SMS == nil {
		return nil
	}
	return &out
}

// gateway reads a vendor gateway from <PREFIX>_URL, _API_KEY and _SECRET.
func gateway(prefix string) *vendors.Gateway {
	u := os.Getenv(prefix + "_URL")
	if u == "" {
		return nil
	}
	if !strings.HasPrefix(u, "https://") || os.Getenv(prefix+"_SECRET") == "" {
		log.Fatalf("%s_URL must be https and %s_SECRET is required", prefix, prefix)
	}
	return &vendors.Gateway{BaseURL: u, APIKey: os.Getenv(prefix + "_API_KEY"), Secret: os.Getenv(prefix + "_SECRET")}
}

// routes is the public HTTP surface. Every route here is covered by the route security
// test (routes_test.go), driven from the OpenAPI contracts.
func routes(a *app) *http.ServeMux {
	srv, prof, safety, commerce, notifier, rights, stores := a.srv, a.prof, a.safety, a.commerce, a.notifier, a.rights, a.stores
	root := http.NewServeMux()
	root.Handle("/v1/profile", prof)
	root.Handle("/v1/profile/", prof)
	root.Handle("/v1/tonight", prof)
	root.Handle("/v1/tonight/", prof)
	root.Handle("/v1/threads", prof)
	root.Handle("/v1/threads/", prof)
	root.Handle("/v1/devices/", prof)
	root.Handle("/v1/meets/", prof)
	root.Handle("/v1/safety/", prof)
	root.Handle("/v1/reports", safety)
	root.Handle("/v1/reports/", safety)
	root.Handle("/v1/appeals", safety)
	root.Handle("/v1/me/standing", safety)
	root.Handle("/ops/v1/", safety)
	root.Handle("/ops/v1/vouchers", commerce)
	root.Handle("/v1/account", rights)
	root.Handle("/v1/account/export", rights)
	root.Handle("/v1/deletion-proofs/", rights)
	root.Handle("/v1/notices", notifier)
	root.Handle("/v1/notices/", notifier)
	root.Handle("/v1/notifications/", notifier)
	for _, p := range []string{"/v1/entitlements", "/v1/deposit", "/v1/keys/", "/v1/subscription", "/v1/subscription/", "/v1/vouchers/", "/v1/billing/"} {
		root.Handle(p, commerce)
	}
	root.Handle("/", srv)
	// Readiness: every database answers.
	root.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		for name, st := range stores {
			if p, ok := st.(interface{ Ping(context.Context) error }); ok {
				if err := p.Ping(ctx); err != nil {
					log.Printf("readyz: %s database unreachable", name)
					http.Error(w, "not ready", http.StatusServiceUnavailable)
					return
				}
			}
		}
		_, _ = w.Write([]byte("ok"))
	})

	return root
}

// apiHeaders adds a locked-down content policy to every API response: the API serves JSON
// and images only, so nothing it returns may run script or be framed, even if opened
// directly in a browser.
func apiHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; sandbox")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
