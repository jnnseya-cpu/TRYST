// Package assurance chooses how a V2 identity check runs, from the providers' certification
// records (docs/spec/03_Backend.md §7.7 A; FR-101, FR-112, D-44, D-49).
//
// TRYST never relies on a provider's word. Each provider's certificates are recorded with
// their scheme and expiry, and the route is computed from what is valid today:
//
//   - A provider may run V2 only with a valid presentation-attack-detection certificate
//     (ISO/IEC 30107-3 tested at iBeta Level 2 or equivalent). None valid → V2 is unavailable:
//     the check fails closed (no-go G-NG-4) rather than running uncertified.
//   - Injection attacks (virtual cameras, deepfake feeds): if the provider also holds a valid
//     injection-attack-detection certificate (CEN/TS 18099 or an equivalent independent test),
//     V2 may run in the web app; otherwise only in the native app with platform attestation
//     (App Attest / Play Integrity), which keeps virtual cameras out; the web hands off to the
//     phone by QR.
//   - Two providers: the better-certified one first; the other is the fallback.
package assurance

import (
	"errors"
	"sort"
	"time"
)

// Scheme of a certificate.
type Scheme string

const (
	PADLevel2 Scheme = "pad_iso30107_3_level2"  // presentation attack detection
	Injection Scheme = "injection_cen_ts_18099" // injection attack detection
)

// Cert is one certificate on record, with the evidence reference checked by Trust & Safety.
type Cert struct {
	Scheme     Scheme
	Lab        string
	Evidence   string // report reference held by T&S
	ValidUntil time.Time
}

// Provider is an age-assurance / identity provider.
type Provider struct {
	Name  string
	Certs []Cert
	Up    bool // provider health
}

func (p Provider) has(s Scheme, now time.Time) bool {
	for _, c := range p.Certs {
		if c.Scheme == s && c.Evidence != "" && now.Before(c.ValidUntil) {
			return true
		}
	}
	return false
}

// Surface the member is on.
type Surface string

const (
	NativeApp Surface = "native_app"
	Web       Surface = "web"
)

// Route is the decision.
type Route struct {
	Provider  string
	Surface   Surface // where the capture must happen
	HandOff   bool    // web member must continue on their phone (QR)
	Attest    bool    // platform attestation required for the capture
	Available bool
	Reason    string
}

var ErrNoCertifiedProvider = errors.New("no provider with a valid liveness certificate: V2 unavailable")

// Choose returns the route for a member on surface, or ErrNoCertifiedProvider.
func Choose(providers []Provider, on Surface, now time.Time) (Route, error) {
	var ok []Provider
	for _, p := range providers {
		if p.Up && p.has(PADLevel2, now) {
			ok = append(ok, p)
		}
	}
	if len(ok) == 0 {
		return Route{Reason: "no provider holds a valid liveness certificate"}, ErrNoCertifiedProvider
	}
	sort.SliceStable(ok, func(i, j int) bool { return ok[i].has(Injection, now) && !ok[j].has(Injection, now) })
	best := ok[0]
	if best.has(Injection, now) {
		return Route{Provider: best.Name, Surface: on, Attest: on == NativeApp, Available: true,
			Reason: "provider certified for liveness and injection detection"}, nil
	}
	r := Route{Provider: best.Name, Surface: NativeApp, Attest: true, Available: true,
		Reason: "no valid injection certificate: native app capture with platform attestation only"}
	if on == Web {
		r.HandOff = true
	}
	return r, nil
}

// Expiring lists certificates that lapse within the warning window, for the API Integration
// agent's renewal reminders (09 §5.5).
func Expiring(providers []Provider, now time.Time, within time.Duration) []string {
	var out []string
	for _, p := range providers {
		for _, c := range p.Certs {
			if c.ValidUntil.After(now) && c.ValidUntil.Sub(now) <= within {
				out = append(out, p.Name+":"+string(c.Scheme))
			}
		}
	}
	return out
}
