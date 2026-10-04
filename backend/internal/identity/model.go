// Package identity is the TRYST identity service: sign-up, passkey registration and
// account-holder-only two-factor login (docs/spec/02_Shared_Contracts.md §3.3; 03 §6.7;
// FR-056..FR-062, D-16).
//
// Storage is behind Store so the in-memory implementation used here can be replaced by the
// IDENTITY-DB (PostgreSQL) adapter without touching the handlers.
package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/jnnseya-cpu/tryst/backend/internal/auth"
	"github.com/jnnseya-cpu/tryst/backend/internal/gate"
)

// Credential kinds (02 §4.1 authenticator.kind).
const (
	KindPlatformBiometric = "platform_biometric" // native, attested biometric-only (not reachable from web)
	KindPlatformUV        = "platform_uv"        // web platform authenticator with user verification
	KindRoamingKey        = "roaming_key"        // FIDO2 security key
)

// StoredCredential is one registered passkey.
type StoredCredential struct {
	Cred      webauthn.Credential
	Surface   auth.Surface
	Kind      string
	CreatedAt time.Time
	Revoked   bool
}

// Identity is an account. It holds no name, no contact in clear and no biometric data.
type Identity struct {
	Handle       []byte // WebAuthn user handle: 32 random bytes
	ContactHash  string // HMAC(pepper, normalised contact)
	Tier         gate.Tier
	Jurisdiction string
	Sealed       bool // true after the first full two-factor login; device adds then need step-up
	Credentials  []*StoredCredential
	Failures     int
	LockedUntil  time.Time
	CreatedAt    time.Time
}

// ID is the hex form of the handle.
func (i *Identity) ID() string { return hex.EncodeToString(i.Handle) }

// webauthnUser adapts Identity to webauthn.User. Names are neutral: they appear in the
// device's passkey manager, so they must not reveal anything (A1).
type webauthnUser struct{ id *Identity }

func (u webauthnUser) WebAuthnID() []byte          { return u.id.Handle }
func (u webauthnUser) WebAuthnName() string        { return "member" }
func (u webauthnUser) WebAuthnDisplayName() string { return "Member" }
func (u webauthnUser) WebAuthnCredentials() []webauthn.Credential {
	out := make([]webauthn.Credential, 0, len(u.id.Credentials))
	for _, c := range u.id.Credentials {
		if !c.Revoked {
			out = append(out, c.Cred)
		}
	}
	return out
}

// SessionKind distinguishes onboarding sessions (sign-up device only, may register passkeys)
// from full sessions issued after two-factor login.
type SessionKind string

const (
	Onboarding SessionKind = "onboarding"
	Full       SessionKind = "full"
)

// Session is server-side; only the SHA-256 of the bearer token is stored.
type Session struct {
	IdentityID string
	Kind       SessionKind
	Surface    auth.Surface
	ExpiresAt  time.Time
	IdleFor    time.Duration
	LastSeen   time.Time
}

// LoginAttempt is an in-flight two-factor login.
type LoginAttempt struct {
	IdentityID  string
	Attempt     *auth.Attempt
	FirstCredID []byte
	Assertion   *webauthn.SessionData // pending ceremony (first or second key)
	QRToken     string
	Approved    bool
	Denied      bool
	Delivered   bool
	ExpiresAt   time.Time
}

type otpEntry struct {
	CodeHash  string
	Attempts  int
	ExpiresAt time.Time
}

type enrolEntry struct {
	IdentityID string
	ExpiresAt  time.Time
}

type approvalEntry struct {
	LoginID   string
	Assertion *webauthn.SessionData
	ExpiresAt time.Time
}

// Store is the persistence boundary.
type Store struct {
	mu          sync.Mutex
	identities  map[string]*Identity // by ID
	byContact   map[string]string    // contact hash -> ID
	sessions    map[string]*Session  // token hash -> session
	regPending  map[string]*webauthn.SessionData
	logins      map[string]*LoginAttempt
	otps        map[string]*otpEntry // contact hash -> otp
	enrols      map[string]*enrolEntry
	approvals   map[string]*approvalEntry // qr token -> approval
	startCounts map[string][]time.Time    // contact hash -> recent OTP sends
}

func NewStore() *Store {
	return &Store{
		identities: map[string]*Identity{}, byContact: map[string]string{}, sessions: map[string]*Session{},
		regPending: map[string]*webauthn.SessionData{}, logins: map[string]*LoginAttempt{}, otps: map[string]*otpEntry{},
		enrols: map[string]*enrolEntry{}, approvals: map[string]*approvalEntry{}, startCounts: map[string][]time.Time{},
	}
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func tokenHash(t string) string {
	s := sha256.Sum256([]byte(t))
	return hex.EncodeToString(s[:])
}

func mustDecodeB64(s string) []byte {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
