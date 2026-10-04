// Package auth implements account-holder-only login (docs/spec/02_Shared_Contracts.md §3.3;
// FR-056..FR-062, D-16).
//
// App and mobile PWA: B1 (device-bound biometric passkey) + B2 (liveness face match).
// Desktop: F1 (passkey) + F2 (approval from a registered phone via B1, or a second key).
// A session exists only once every required factor has passed, in order.
package auth

import (
	"errors"
	"time"
)

// Surface is decided by the server from attestation and authenticator properties, never by
// a client claim (downgrade resistance).
type Surface string

const (
	MobileApp Surface = "mobile_app"
	MobilePWA Surface = "mobile_pwa"
	Desktop   Surface = "desktop"
)

// Factor names follow the spec.
type Factor string

const (
	B1 Factor = "B1" // device biometric passkey
	B2 Factor = "B2" // liveness face match
	F1 Factor = "F1" // desktop passkey
	F2 Factor = "F2" // registered-phone approval or second security key
)

// Never-accepted factors exist only so tests can prove they are rejected.
const (
	SMSOTP      Factor = "sms_otp"
	EmailOTP    Factor = "email_otp"
	MagicLink   Factor = "magic_link"
	StaffAssist Factor = "staff_override"
)

var (
	ErrWrongFactor     = errors.New("second_factor_required")
	ErrNotAcceptedType = errors.New("factor type is never accepted for login")
	ErrExpired         = errors.New("login attempt expired")
	ErrInvalidated     = errors.New("authenticator_invalidated")
)

// RequiredFactors returns the ordered factors for a surface. Always exactly two.
func RequiredFactors(s Surface) []Factor {
	switch s {
	case MobileApp, MobilePWA:
		return []Factor{B1, B2}
	default: // Desktop and anything unrecognised take the desktop path, which needs a second registered authenticator.
		return []Factor{F1, F2}
	}
}

// SessionPolicy is per-surface session lifetime (FR-062).
func SessionPolicy(s Surface) (max, idle time.Duration) {
	if s == Desktop {
		return 12 * time.Hour, 15 * time.Minute
	}
	return 30 * 24 * time.Hour, 2 * time.Minute
}

// Attempt is one in-flight login. It lives in Redis with a 5 minute TTL in production.
type Attempt struct {
	Surface   Surface
	StartedAt time.Time
	passed    []Factor
}

const attemptTTL = 5 * time.Minute

// NewAttempt starts a login on a server-decided surface.
func NewAttempt(s Surface, now time.Time) *Attempt { return &Attempt{Surface: s, StartedAt: now} }

// Pass records a verified factor. Factors must arrive in the required order.
func (a *Attempt) Pass(f Factor, now time.Time) error {
	switch f {
	case SMSOTP, EmailOTP, MagicLink, StaffAssist:
		return ErrNotAcceptedType
	}
	if now.Sub(a.StartedAt) > attemptTTL {
		return ErrExpired
	}
	req := RequiredFactors(a.Surface)
	if len(a.passed) >= len(req) || req[len(a.passed)] != f {
		return ErrWrongFactor
	}
	a.passed = append(a.passed, f)
	return nil
}

// Complete reports whether a session may be issued.
func (a *Attempt) Complete() bool { return len(a.passed) == len(RequiredFactors(a.Surface)) }

// Authenticator is a registered WebAuthn credential.
type Authenticator struct {
	CredentialID      string
	BioEnrolmentBound bool
	EnrolmentDigest   string // platform-reported biometric-set state at registration
	Revoked           bool
}

// CheckEnrolment invalidates a B1 credential when the device's biometric set changed
// (someone added a fingerprint or face) — FR-059.
func (au *Authenticator) CheckEnrolment(currentDigest string) error {
	if au.Revoked {
		return ErrInvalidated
	}
	if au.BioEnrolmentBound && currentDigest != au.EnrolmentDigest {
		au.Revoked = true
		return ErrInvalidated
	}
	return nil
}

// StepUpRequired lists sensitive actions that need both factors again (02 §3.3). Safety and
// exit actions are deliberately absent and must never be gated.
var StepUpRequired = map[string]bool{
	"device_add": true, "device_revoke": true, "device_list": true,
	"discretion_broaden": true, "contact_change": true, "payment_change": true,
	"couple_cosign": true, "veto_change": true, "data_export": true, "recovery_start": true,
}

// NeverGated must stay one step: Burn, Panic, report, block, cancel, delete.
var NeverGated = []string{"burn", "panic", "report", "block", "subscription_cancel", "account_delete"}
