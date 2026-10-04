// Command identity-svc runs the TRYST identity service: sign-up, passkey registration and
// account-holder-only two-factor login (docs/spec/02_Shared_Contracts.md §3.3).
//
// Environment:
//
//	TRYST_ENV       dev | prod (default prod; prod refuses dev shortcuts)
//	RP_ID           WebAuthn relying-party ID (e.g. localhost, app.example.com)
//	RP_ORIGINS      comma-separated allowed origins (e.g. http://localhost:3000)
//	CONTACT_PEPPER  hex, >= 32 bytes in prod (HSM-held in production)
//	PORT            default 8081
package main

import (
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
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
	if p := os.Getenv("CONTACT_PEPPER"); p != "" {
		b, err := hex.DecodeString(p)
		if err != nil {
			log.Fatal("CONTACT_PEPPER must be hex")
		}
		cfg.ContactPepper = b
	}
	if mode == "dev" {
		cfg.Liveness = identity.DevLiveness{}
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
	addr := ":" + env("PORT", "8081")
	log.Printf("identity-svc (%s) listening on %s", mode, addr)
	hs := &http.Server{Addr: addr, Handler: srv, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(hs.ListenAndServe())
}
