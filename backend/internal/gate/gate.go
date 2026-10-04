// Package gate holds edge middleware: jurisdiction and verification-tier gates
// (docs/spec/02_Shared_Contracts.md §3.2, §5; FR-001, FR-043).
package gate

import (
	"context"
	"net/http"

	"github.com/jnnseya-cpu/tryst/backend/internal/problem"
)

// Tier is a verification tier; higher is stronger.
type Tier int

const (
	V0 Tier = iota
	V1
	V2
	V3
)

func (t Tier) String() string { return [...]string{"V0", "V1", "V2", "V3"}[t] }

// Claims are what the verified, DPoP-bound access token asserts about the caller.
type Claims struct {
	SubjectRef   string
	Tier         Tier
	Jurisdiction string
	Art9Consent  bool
}

type ctxKey struct{}

// WithClaims attaches claims; the auth layer calls this after token verification.
func WithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// ClaimsFrom returns the caller's claims, if any.
func ClaimsFrom(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(Claims)
	return c, ok
}

// Jurisdictions is the enabled-market list (launch: GB, IE). Each entry must reference a
// written legal opinion in configuration (01 §14.5).
type Jurisdictions map[string]bool

// Jurisdiction blocks callers outside enabled markets.
func Jurisdiction(allowed Jurisdictions, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := ClaimsFrom(r.Context())
		if !ok {
			problem.Write(w, problem.Problem{Title: "Authentication required", Status: http.StatusUnauthorized, Code: "auth_required"})
			return
		}
		if !allowed[c.Jurisdiction] {
			problem.Write(w, problem.Problem{Title: "Not available here", Status: http.StatusForbidden, Code: "jurisdiction_blocked"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireTier enforces the minimum verification tier for an operation (x-min-tier).
func RequireTier(min Tier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := ClaimsFrom(r.Context())
		if !ok {
			problem.Write(w, problem.Problem{Title: "Authentication required", Status: http.StatusUnauthorized, Code: "auth_required"})
			return
		}
		if c.Tier < min {
			problem.Write(w, problem.Problem{Title: "Verification required", Status: http.StatusForbidden, Code: "tier_required", RequiredTier: min.String()})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireArt9Consent blocks special-category processing without explicit consent.
func RequireArt9Consent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, ok := ClaimsFrom(r.Context()); !ok || !c.Art9Consent {
			problem.Write(w, problem.Problem{Title: "Consent required", Status: http.StatusForbidden, Code: "consent_required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
