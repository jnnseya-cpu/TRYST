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
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/profilesvc"
	"github.com/jnnseya-cpu/tryst/backend/internal/safetysvc"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
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

func main() {
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
	ctx := context.Background()
	idStore := openStore(ctx, mode, "DATABASE_URL_IDENTITY")
	profStore := openStore(ctx, mode, "DATABASE_URL_PROFILE")
	keyStore := openStore(ctx, mode, "DATABASE_URL_KEYS")
	vault := openVault(mode, keyStore)
	cfg.Persist = idStore
	srv, err := identity.New(cfg, identity.NewStore())
	if err != nil {
		log.Fatal(err)
	}
	var mod profilesvc.Moderator = profilesvc.NoModerator{}
	if mode == "dev" {
		mod = profilesvc.DevModerator{}
		log.Printf("DEV MODE: photo moderation vendors are simulated; AI-label checks run for real.")
	}
	prof, err := profilesvc.NewWithStore(srv, mod, mode, profStore, vault)
	if err != nil {
		log.Fatal(err)
	}
	if mode == "dev" {
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
	safety, err := safetysvc.New(srv, staff, prof, openStore(ctx, mode, "DATABASE_URL_SAFETY"))
	if err != nil {
		log.Fatal(err)
	}
	prof.OnSignal = safety.FromSignal
	go func() {
		for range time.Tick(30 * time.Second) {
			prof.SweepSafety(context.Background())
		}
	}()
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
	root.Handle("/", srv)
	addr := ":" + env("PORT", "8081")
	log.Printf("identity-svc (%s) listening on %s", mode, addr)
	hs := &http.Server{Addr: addr, Handler: root, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(hs.ListenAndServe())
}
