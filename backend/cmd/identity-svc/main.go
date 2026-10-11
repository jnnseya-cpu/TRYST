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
package main

import (
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/profilesvc"
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
	srv, err := identity.New(cfg, identity.NewStore())
	if err != nil {
		log.Fatal(err)
	}
	var mod profilesvc.Moderator = profilesvc.NoModerator{}
	if mode == "dev" {
		mod = profilesvc.DevModerator{}
		log.Printf("DEV MODE: photo moderation vendors are simulated; AI-label checks run for real.")
	}
	prof, err := profilesvc.New(srv, mod, mode)
	if err != nil {
		log.Fatal(err)
	}
	root := http.NewServeMux()
	root.Handle("/v1/profile", prof)
	root.Handle("/v1/profile/", prof)
	root.Handle("/v1/tonight", prof)
	root.Handle("/v1/tonight/", prof)
	root.Handle("/", srv)
	addr := ":" + env("PORT", "8081")
	log.Printf("identity-svc (%s) listening on %s", mode, addr)
	hs := &http.Server{Addr: addr, Handler: root, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(hs.ListenAndServe())
}
