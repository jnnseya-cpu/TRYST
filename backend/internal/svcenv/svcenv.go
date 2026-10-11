// Package svcenv holds the small pieces of start-up configuration every TRYST service
// shares: environment lookups, per-domain stores, internal-call secrets and listeners.
package svcenv

import (
	"context"
	"crypto/sha256"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/store"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcauth"
)

// Env returns the variable or a default.
func Env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// OpenStore connects to the database named by envVar. Production refuses to run without
// it; dev falls back to memory.
func OpenStore(ctx context.Context, mode, envVar string) store.Store {
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

// InternalSecret reads a hex secret (≥ 32 bytes) for one direction of internal calls.
// Dev uses a fixed, publicly known value so local services find each other; production
// refuses to start without a real one.
func InternalSecret(mode, envVar string) []byte {
	if v := os.Getenv(envVar); v != "" {
		b, err := svcauth.Secret(v)
		if err != nil {
			log.Fatalf("%s: %v", envVar, err)
		}
		return b
	}
	if mode != "dev" {
		log.Fatalf("%s (hex, 32 bytes) is required in production", envVar)
	}
	h := sha256.Sum256([]byte("tryst-dev-only/" + envVar))
	return h[:]
}

// PreviousSecret reads an optional rotation secret (<envVar>_PREVIOUS); nil if unset.
func PreviousSecret(envVar string) []byte {
	v := os.Getenv(envVar + "_PREVIOUS")
	if v == "" || v == "none" { // "none": provisioned placeholder outside a rotation
		return nil
	}
	b, err := svcauth.Secret(v)
	if err != nil {
		log.Fatalf("%s_PREVIOUS: %v", envVar, err)
	}
	return b
}

// Verifier accepts calls from caller signed with envVar's secret (or, during a rotation,
// with <envVar>_PREVIOUS).
func Verifier(mode, caller, envVar string) *svcauth.Verifier {
	v := &svcauth.Verifier{Secrets: map[string][]byte{caller: InternalSecret(mode, envVar)}}
	if p := PreviousSecret(envVar); p != nil {
		v.Previous = map[string][]byte{caller: p}
	}
	return v
}

// ServeInternal runs an internal listener (never exposed publicly).
func ServeInternal(addr string, h http.Handler) {
	go func() {
		srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second}
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("internal listener %s: %v", addr, err)
		}
	}()
}

// APIHeaders adds a locked-down content policy to every API response: APIs serve JSON and
// images only, so nothing they return may run script or be framed.
func APIHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; sandbox")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
