package identity

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/jnnseya-cpu/tryst/backend/internal/auth"
	"github.com/jnnseya-cpu/tryst/backend/internal/gate"
	"github.com/jnnseya-cpu/tryst/backend/internal/problem"
)

const (
	maxBody             = 64 << 10
	onboardingTTL       = time.Hour
	loginTTL            = 5 * time.Minute
	qrTTL               = 60 * time.Second
	enrolTTL            = 10 * time.Minute
	maxOnboardingCreds  = 5
	lockoutThreshold    = 5
	lockoutDuration     = 15 * time.Minute
	perIPRequestsPerMin = 120
)

// Config configures the identity service.
type Config struct {
	RPID          string
	RPDisplayName string
	RPOrigins     []string
	Env           string // "dev" or "prod"
	ContactPepper []byte // HSM-held in production
	Liveness      LivenessProvider
	Sender        Sender
	Jurisdictions gate.Jurisdictions
	Now           func() time.Time
}

// Server is the identity HTTP service.
type Server struct {
	cfg   Config
	wa    *webauthn.WebAuthn
	store *Store
	mux   *http.ServeMux
	rl    map[string][]time.Time
}

// New validates configuration. It refuses dev-only shortcuts outside the dev environment.
func New(cfg Config, store *Store) (*Server, error) {
	if cfg.Env != "dev" && cfg.Env != "prod" {
		return nil, errors.New("Env must be dev or prod")
	}
	if cfg.Liveness == nil {
		cfg.Liveness = NoLiveness{}
	}
	if _, isDev := cfg.Liveness.(DevLiveness); isDev && cfg.Env != "dev" {
		return nil, errors.New("DevLiveness is forbidden outside the dev environment")
	}
	if cfg.Env != "dev" && len(cfg.ContactPepper) < 32 {
		return nil, errors.New("ContactPepper must be at least 32 bytes in prod")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Jurisdictions == nil {
		cfg.Jurisdictions = gate.Jurisdictions{"GB": true, "IE": true}
	}
	wa, err := webauthn.New(&webauthn.Config{RPID: cfg.RPID, RPDisplayName: cfg.RPDisplayName, RPOrigins: cfg.RPOrigins})
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, wa: wa, store: store, mux: http.NewServeMux(), rl: map[string][]time.Time{}}
	s.routes()
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if strings.HasPrefix(r.URL.Path, "/v1/") && !s.allow(r) {
		w.Header().Set("Retry-After", "60")
		fail(w, http.StatusTooManyRequests, "rate_limited", "Too many requests")
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("POST /v1/auth/start", s.handleStart)
	m.HandleFunc("POST /v1/auth/verify", s.handleVerify)
	m.HandleFunc("POST /v1/auth/passkeys/register/begin", s.handleRegisterBegin)
	m.HandleFunc("POST /v1/auth/passkeys/register/finish", s.handleRegisterFinish)
	m.HandleFunc("POST /v1/auth/enrol-device", s.handleEnrolBegin)
	m.HandleFunc("POST /v1/auth/enrol-device/redeem", s.handleEnrolRedeem)
	m.HandleFunc("POST /v1/auth/login/begin", s.handleLoginBegin)
	m.HandleFunc("POST /v1/auth/login/passkey", s.handleLoginPasskey)
	m.HandleFunc("POST /v1/auth/login/liveness", s.handleLoginLiveness)
	m.HandleFunc("POST /v1/auth/login/cross-device", s.handleCrossDeviceBegin)
	m.HandleFunc("GET /v1/auth/login/cross-device/{qr}", s.handleCrossDeviceStatus)
	m.HandleFunc("POST /v1/auth/approvals/{qr}/begin", s.handleApprovalBegin)
	m.HandleFunc("POST /v1/auth/approvals/{qr}", s.handleApprovalFinish)
	m.HandleFunc("POST /v1/auth/login/second-key/begin", s.handleSecondKeyBegin)
	m.HandleFunc("POST /v1/auth/login/second-key", s.handleSecondKeyFinish)
	m.HandleFunc("GET /v1/me", s.handleMe)
	m.HandleFunc("POST /v1/auth/logout", s.handleLogout)
	m.HandleFunc("POST /v1/burn", s.handleBurn)
}

// ---------------------------------------------------------------- helpers

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, code, title string) {
	problem.Write(w, problem.Problem{Title: title, Status: status, Code: code})
}

// genericLoginFailure never says which factor failed (04 §4.0).
func genericLoginFailure(w http.ResponseWriter) {
	fail(w, http.StatusUnauthorized, "auth_required", "We couldn't sign you in. Please try again.")
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (s *Server) allow(r *http.Request) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	now := s.cfg.Now()
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	cut := now.Add(-time.Minute)
	kept := s.rl[ip][:0]
	for _, t := range s.rl[ip] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= perIPRequestsPerMin {
		s.rl[ip] = kept
		return false
	}
	s.rl[ip] = append(kept, now)
	return true
}

var mobileUA = regexp.MustCompile(`(?i)Android|iPhone|iPad|iPod|Mobile`)

// surfaceForRegistration records which login path a credential belongs to. Platform passkeys
// on a mobile browser take the mobile path (B1+B2); everything else takes the desktop path
// (F1+F2). Both paths need two factors, so a spoofed user agent gains nothing: claiming
// "mobile" adds a liveness match, claiming "desktop" needs a second registered authenticator.
func surfaceForRegistration(r *http.Request, attachment protocol.AuthenticatorAttachment) (auth.Surface, string) {
	if attachment == protocol.CrossPlatform {
		return auth.Desktop, KindRoamingKey
	}
	if mobileUA.MatchString(r.UserAgent()) {
		return auth.MobilePWA, KindPlatformUV
	}
	return auth.Desktop, KindPlatformUV
}

func factorsFor(s auth.Surface) []string {
	f := auth.RequiredFactors(s)
	return []string{string(f[0]), string(f[1])}
}

// sessionFrom authenticates a bearer token. Caller must hold the store lock.
func (s *Server) sessionFrom(r *http.Request) (*Session, string, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil, "", false
	}
	th := tokenHash(strings.TrimPrefix(h, "Bearer "))
	sess, ok := s.store.sessions[th]
	if !ok {
		return nil, "", false
	}
	now := s.cfg.Now()
	if now.After(sess.ExpiresAt) || (sess.IdleFor > 0 && now.Sub(sess.LastSeen) > sess.IdleFor) {
		delete(s.store.sessions, th)
		return nil, "", false
	}
	sess.LastSeen = now
	return sess, th, true
}

func (s *Server) issueSession(id *Identity, kind SessionKind, surface auth.Surface) map[string]any {
	now := s.cfg.Now()
	tok := randomToken(32)
	sess := &Session{IdentityID: id.ID(), Kind: kind, Surface: surface, LastSeen: now}
	if kind == Onboarding {
		sess.ExpiresAt = now.Add(onboardingTTL)
	} else {
		max, idle := auth.SessionPolicy(surface)
		sess.ExpiresAt, sess.IdleFor = now.Add(max), idle
	}
	s.store.sessions[tokenHash(tok)] = sess
	return map[string]any{"access_token": tok, "expires_at": sess.ExpiresAt.UTC().Format(time.RFC3339), "surface": surface, "kind": kind}
}

func (s *Server) recordFailure(id *Identity) {
	id.Failures++
	if id.Failures >= lockoutThreshold {
		id.LockedUntil = s.cfg.Now().Add(lockoutDuration)
		id.Failures = 0
	}
}

func (s *Server) locked(id *Identity) bool { return s.cfg.Now().Before(id.LockedUntil) }

// ---------------------------------------------------------------- sign-up (contact OTP, V0)

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Contact string `json:"contact"`
	}
	if err := decode(r, &req); err != nil || !ValidContact(NormaliseContact(req.Contact)) {
		fail(w, 422, "validation_failed", "Enter a valid email or phone number in international format")
		return
	}
	ch := contactHash(s.cfg.ContactPepper, req.Contact)
	now := s.cfg.Now()
	s.store.mu.Lock()
	sends := s.store.startCounts[ch][:0]
	for _, t := range s.store.startCounts[ch] {
		if now.Sub(t) < time.Hour {
			sends = append(sends, t)
		}
	}
	if len(sends) >= otpMaxSends {
		s.store.startCounts[ch] = sends
		s.store.mu.Unlock()
		w.Header().Set("Retry-After", "3600")
		fail(w, 429, "rate_limited", "Too many codes requested")
		return
	}
	s.store.startCounts[ch] = append(sends, now)
	code := newOTP()
	s.store.otps[ch] = &otpEntry{CodeHash: tokenHash(ch + ":" + code), ExpiresAt: now.Add(otpTTL)}
	s.store.mu.Unlock()

	if s.cfg.Sender != nil {
		if err := s.cfg.Sender.Send(NormaliseContact(req.Contact), code); err != nil {
			log.Printf("otp send failed: %v", err) // never log the contact or the code
		}
	}
	if s.cfg.Env == "dev" {
		w.Header().Set("X-Dev-OTP", code)
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sent"}) // uniform: never reveals whether an account exists
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Contact      string `json:"contact"`
		OTP          string `json:"otp"`
		Jurisdiction string `json:"jurisdiction"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, 422, "validation_failed", "Malformed request")
		return
	}
	jur := strings.ToUpper(strings.TrimSpace(req.Jurisdiction))
	if jur == "" {
		jur = "GB"
	}
	if !s.cfg.Jurisdictions[jur] {
		fail(w, 403, "jurisdiction_blocked", "TRYST is not available here")
		return
	}
	ch := contactHash(s.cfg.ContactPepper, req.Contact)
	now := s.cfg.Now()
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	e, ok := s.store.otps[ch]
	if !ok || now.After(e.ExpiresAt) || e.Attempts >= otpMaxAttempts {
		fail(w, 422, "validation_failed", "That code is invalid or has expired")
		return
	}
	e.Attempts++
	if subtle.ConstantTimeCompare([]byte(e.CodeHash), []byte(tokenHash(ch+":"+strings.TrimSpace(req.OTP)))) != 1 {
		fail(w, 422, "validation_failed", "That code is invalid or has expired")
		return
	}
	delete(s.store.otps, ch)

	if existingID, exists := s.store.byContact[ch]; exists {
		id := s.store.identities[existingID]
		// An OTP can never add a passkey to an account that already has one: that would make
		// the contact handle a login factor (FR-056). Members sign in with passkeys.
		if id.Sealed || len(id.Credentials) > 0 {
			fail(w, 409, "account_exists_sign_in", "You already have an account. Sign in with your passkey.")
			return
		}
		writeJSON(w, 200, s.issueSession(id, Onboarding, ""))
		return
	}
	handle := mustDecodeB64(randomToken(32)) // 32 random bytes; never derived from the contact
	id := &Identity{Handle: handle, ContactHash: ch, Tier: gate.V0, Jurisdiction: jur, CreatedAt: now}
	s.store.identities[id.ID()] = id
	s.store.byContact[ch] = id.ID()
	writeJSON(w, 201, s.issueSession(id, Onboarding, ""))
}

// ---------------------------------------------------------------- passkey registration

// canRegister: onboarding sessions only (before the account is sealed). Adding a device to a
// sealed account needs step-up (02 §3.3), which is not part of this iteration.
func (s *Server) canRegister(sess *Session, id *Identity) bool {
	return sess.Kind == Onboarding && !id.Sealed
}

func (s *Server) handleRegisterBegin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Attachment string `json:"attachment"` // "platform" (default) or "cross-platform"
	}
	_ = decode(r, &req)
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	sess, th, ok := s.sessionFrom(r)
	if !ok {
		fail(w, 401, "auth_required", "Sign in required")
		return
	}
	id := s.store.identities[sess.IdentityID]
	if !s.canRegister(sess, id) {
		fail(w, 403, "step_up_required", "Adding a device to an existing account needs both sign-in checks")
		return
	}
	if len(id.Credentials) >= maxOnboardingCreds {
		fail(w, 422, "validation_failed", "Too many passkeys")
		return
	}
	attach := protocol.Platform
	if req.Attachment == string(protocol.CrossPlatform) {
		attach = protocol.CrossPlatform
	}
	user := webauthnUser{id}
	excl := make([]protocol.CredentialDescriptor, 0, len(id.Credentials))
	for _, c := range user.WebAuthnCredentials() {
		excl = append(excl, c.Descriptor())
	}
	creation, data, err := s.wa.BeginRegistration(user,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			AuthenticatorAttachment: attach,
			ResidentKey:             protocol.ResidentKeyRequirementRequired,
			RequireResidentKey:      protocol.ResidentKeyRequired(),
			UserVerification:        protocol.VerificationRequired,
		}),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
		webauthn.WithExclusions(excl),
	)
	if err != nil {
		fail(w, 500, "validation_failed", "Could not start registration")
		return
	}
	s.store.regPending[th] = data
	writeJSON(w, 200, creation)
}

func (s *Server) handleRegisterFinish(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fail(w, 422, "validation_failed", "Malformed request")
		return
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(body)
	if err != nil {
		fail(w, 422, "validation_failed", "Malformed passkey response")
		return
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	sess, th, ok := s.sessionFrom(r)
	if !ok {
		fail(w, 401, "auth_required", "Sign in required")
		return
	}
	id := s.store.identities[sess.IdentityID]
	data, pending := s.store.regPending[th]
	if !pending || !s.canRegister(sess, id) {
		fail(w, 403, "step_up_required", "Registration not allowed")
		return
	}
	delete(s.store.regPending, th)
	cred, err := s.wa.CreateCredential(webauthnUser{id}, *data, parsed)
	if err != nil || !cred.Flags.UserVerified {
		fail(w, 422, "validation_failed", "Passkey could not be verified")
		return
	}
	surface, kind := surfaceForRegistration(r, parsed.AuthenticatorAttachment)
	id.Credentials = append(id.Credentials, &StoredCredential{Cred: *cred, Surface: surface, Kind: kind, CreatedAt: s.cfg.Now()})
	writeJSON(w, 201, map[string]any{"registered": true, "surface": surface, "kind": kind, "passkeys": len(id.Credentials)})
}

// ---------------------------------------------------------------- phone enrolment during onboarding

func (s *Server) handleEnrolBegin(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	sess, _, ok := s.sessionFrom(r)
	if !ok {
		fail(w, 401, "auth_required", "Sign in required")
		return
	}
	id := s.store.identities[sess.IdentityID]
	if !s.canRegister(sess, id) {
		fail(w, 403, "step_up_required", "Adding a device to an existing account needs both sign-in checks")
		return
	}
	tok := randomToken(24)
	exp := s.cfg.Now().Add(enrolTTL)
	s.store.enrols[tokenHash(tok)] = &enrolEntry{IdentityID: id.ID(), ExpiresAt: exp}
	writeJSON(w, 201, map[string]any{"enrol_token": tok, "enrol_path": "/enrol?t=" + tok, "expires_at": exp.UTC().Format(time.RFC3339)})
}

func (s *Server) handleEnrolRedeem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EnrolToken string `json:"enrol_token"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, 422, "validation_failed", "Malformed request")
		return
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	th := tokenHash(req.EnrolToken)
	e, ok := s.store.enrols[th]
	delete(s.store.enrols, th) // single use
	if !ok || s.cfg.Now().After(e.ExpiresAt) {
		fail(w, 422, "validation_failed", "This link has expired")
		return
	}
	id := s.store.identities[e.IdentityID]
	if id.Sealed {
		fail(w, 403, "step_up_required", "Adding a device to an existing account needs both sign-in checks")
		return
	}
	writeJSON(w, 200, s.issueSession(id, Onboarding, ""))
}

// ---------------------------------------------------------------- login: first factor (B1 or F1)

func (s *Server) handleLoginBegin(w http.ResponseWriter, _ *http.Request) {
	assertion, data, err := s.wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		fail(w, 500, "auth_required", "Could not start sign-in")
		return
	}
	loginID := randomToken(24)
	s.store.mu.Lock()
	s.store.logins[tokenHash(loginID)] = &LoginAttempt{Assertion: data, ExpiresAt: s.cfg.Now().Add(loginTTL)}
	s.store.mu.Unlock()
	writeJSON(w, 200, map[string]any{"login_id": loginID, "options": assertion})
}

type credentialEnvelope struct {
	LoginID    string          `json:"login_id"`
	Credential json.RawMessage `json:"credential"`
}

// loginFrom loads an unexpired attempt. Caller must hold the lock.
func (s *Server) loginFrom(loginID string) (*LoginAttempt, bool) {
	la, ok := s.store.logins[tokenHash(loginID)]
	if !ok || s.cfg.Now().After(la.ExpiresAt) {
		return nil, false
	}
	return la, true
}

func findCred(id *Identity, credID []byte) *StoredCredential {
	for _, c := range id.Credentials {
		if !c.Revoked && bytes.Equal(c.Cred.ID, credID) {
			return c
		}
	}
	return nil
}

func (s *Server) handleLoginPasskey(w http.ResponseWriter, r *http.Request) {
	var env credentialEnvelope
	if err := decode(r, &env); err != nil {
		fail(w, 422, "validation_failed", "Malformed request")
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(env.Credential)
	if err != nil {
		genericLoginFailure(w)
		return
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	la, ok := s.loginFrom(env.LoginID)
	if !ok || la.Assertion == nil || la.Attempt != nil {
		fail(w, 400, "auth_required", "This sign-in has expired. Start again.")
		return
	}
	var owner *Identity
	handler := func(_, userHandle []byte) (webauthn.User, error) {
		id, ok := s.store.identities[hex.EncodeToString(userHandle)]
		if !ok {
			return nil, errors.New("unknown user")
		}
		owner = id
		return webauthnUser{id}, nil
	}
	_, cred, err := s.wa.ValidatePasskeyLogin(handler, *la.Assertion, parsed)
	la.Assertion = nil // single use
	if owner != nil && s.locked(owner) {
		genericLoginFailure(w)
		return
	}
	if err != nil || owner == nil || !cred.Flags.UserVerified || cred.Authenticator.CloneWarning {
		if owner != nil {
			s.recordFailure(owner)
		}
		genericLoginFailure(w)
		return
	}
	sc := findCred(owner, cred.ID)
	if sc == nil {
		genericLoginFailure(w)
		return
	}
	sc.Cred.Authenticator, sc.Cred.Flags = cred.Authenticator, cred.Flags

	first := auth.F1
	if sc.Surface != auth.Desktop {
		first = auth.B1
	}
	la.IdentityID, la.FirstCredID = owner.ID(), sc.Cred.ID
	la.Attempt = auth.NewAttempt(sc.Surface, s.cfg.Now())
	if err := la.Attempt.Pass(first, s.cfg.Now()); err != nil {
		genericLoginFailure(w)
		return
	}
	writeJSON(w, 200, map[string]any{"state": "factor1_ok", "surface": sc.Surface, "required_factors": factorsFor(sc.Surface), "next_factor": string(auth.RequiredFactors(sc.Surface)[1])})
}

// finish issues the full session once both factors passed and seals the account.
func (s *Server) finish(w http.ResponseWriter, la *LoginAttempt) {
	id := s.store.identities[la.IdentityID]
	if !la.Attempt.Complete() {
		genericLoginFailure(w)
		return
	}
	// Tier is not raised here: a full session proves the account holder, not age. V1/V2 come
	// from the assurance service (02 §3.2).
	id.Sealed, id.Failures = true, 0
	writeJSON(w, 200, s.issueSession(id, Full, la.Attempt.Surface))
}

// ---------------------------------------------------------------- mobile second factor: B2 liveness

func (s *Server) handleLoginLiveness(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LoginID  string `json:"login_id"`
		Evidence string `json:"evidence"` // provider session reference
	}
	if err := decode(r, &req); err != nil {
		fail(w, 422, "validation_failed", "Malformed request")
		return
	}
	s.store.mu.Lock()
	la, ok := s.loginFrom(req.LoginID)
	if !ok || la.Attempt == nil || la.Attempt.Surface == auth.Desktop || la.Attempt.Complete() {
		s.store.mu.Unlock()
		fail(w, 400, "auth_required", "This sign-in has expired. Start again.")
		return
	}
	identityID := la.IdentityID
	s.store.mu.Unlock()

	match, err := s.cfg.Liveness.Verify(context.Background(), identityID, req.Evidence)

	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if errors.Is(err, ErrLivenessUnavailable) {
		fail(w, 503, "liveness_unavailable", "Live face check is unavailable. Please try again later.")
		return
	}
	id := s.store.identities[identityID]
	if err != nil || !match {
		s.recordFailure(id)
		genericLoginFailure(w)
		return
	}
	if err := la.Attempt.Pass(auth.B2, s.cfg.Now()); err != nil {
		genericLoginFailure(w)
		return
	}
	s.finish(w, la)
}

// ---------------------------------------------------------------- desktop second factor: F2

func (s *Server) handleCrossDeviceBegin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LoginID string `json:"login_id"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, 422, "validation_failed", "Malformed request")
		return
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	la, ok := s.loginFrom(req.LoginID)
	if !ok || la.Attempt == nil || la.Attempt.Surface != auth.Desktop || la.Attempt.Complete() {
		fail(w, 400, "auth_required", "This sign-in has expired. Start again.")
		return
	}
	qr := randomToken(24)
	la.QRToken = tokenHash(qr)
	exp := s.cfg.Now().Add(qrTTL)
	s.store.approvals[tokenHash(qr)] = &approvalEntry{LoginID: tokenHash(req.LoginID), ExpiresAt: exp}
	writeJSON(w, 200, map[string]any{"qr_token": qr, "approve_path": "/approve?qr=" + qr, "expires_at": exp.UTC().Format(time.RFC3339)})
}

// handleCrossDeviceStatus is polled by the desktop. It requires the login_id as well as the QR
// token, so someone who only photographs the QR code cannot collect the session.
func (s *Server) handleCrossDeviceStatus(w http.ResponseWriter, r *http.Request) {
	qr, loginID := r.PathValue("qr"), r.Header.Get("X-Login-Id")
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	la, ok := s.loginFrom(loginID)
	if !ok || la.QRToken == "" || subtle.ConstantTimeCompare([]byte(la.QRToken), []byte(tokenHash(qr))) != 1 {
		fail(w, 404, "not_found", "Not found")
		return
	}
	switch {
	case la.Denied:
		fail(w, 401, "auth_required", "Sign-in was declined on your phone.")
	case la.Approved && !la.Delivered:
		la.Delivered = true
		s.finish(w, la)
	case la.Delivered:
		fail(w, 404, "not_found", "Not found")
	default:
		writeJSON(w, 200, map[string]string{"state": "approval_pending"})
	}
}

func (s *Server) mobileCreds(id *Identity) []protocol.CredentialDescriptor {
	var out []protocol.CredentialDescriptor
	for _, c := range id.Credentials {
		if !c.Revoked && c.Surface != auth.Desktop {
			out = append(out, c.Cred.Descriptor())
		}
	}
	return out
}

func (s *Server) handleApprovalBegin(w http.ResponseWriter, r *http.Request) {
	qr := r.PathValue("qr")
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	ap, ok := s.store.approvals[tokenHash(qr)]
	if !ok || s.cfg.Now().After(ap.ExpiresAt) {
		fail(w, 404, "not_found", "This code has expired")
		return
	}
	la := s.store.logins[ap.LoginID]
	if la == nil || la.Attempt == nil {
		fail(w, 404, "not_found", "This code has expired")
		return
	}
	id := s.store.identities[la.IdentityID]
	allowed := s.mobileCreds(id)
	if len(allowed) == 0 {
		fail(w, 422, "validation_failed", "No registered phone. Use your second security key instead.")
		return
	}
	assertion, data, err := s.wa.BeginLogin(webauthnUser{id}, webauthn.WithAllowedCredentials(allowed), webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		fail(w, 500, "auth_required", "Could not start approval")
		return
	}
	ap.Assertion = data
	writeJSON(w, 200, map[string]any{"options": assertion})
}

func (s *Server) handleApprovalFinish(w http.ResponseWriter, r *http.Request) {
	qr := r.PathValue("qr")
	var req struct {
		Decision   string          `json:"decision"`
		Credential json.RawMessage `json:"credential"`
	}
	if err := decode(r, &req); err != nil || (req.Decision != "approve" && req.Decision != "deny") {
		fail(w, 422, "validation_failed", "Malformed request")
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(req.Credential)
	if err != nil {
		fail(w, 422, "validation_failed", "Malformed passkey response")
		return
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	ap, ok := s.store.approvals[tokenHash(qr)]
	if !ok || ap.Assertion == nil || s.cfg.Now().After(ap.ExpiresAt) {
		fail(w, 404, "not_found", "This code has expired")
		return
	}
	data := ap.Assertion
	delete(s.store.approvals, tokenHash(qr)) // single use
	la := s.store.logins[ap.LoginID]
	if la == nil || la.Attempt == nil {
		fail(w, 404, "not_found", "This code has expired")
		return
	}
	id := s.store.identities[la.IdentityID]
	cred, err := s.wa.ValidateLogin(webauthnUser{id}, *data, parsed)
	if err != nil || !cred.Flags.UserVerified || cred.Authenticator.CloneWarning {
		s.recordFailure(id)
		fail(w, 401, "auth_required", "Approval could not be verified")
		return
	}
	sc := findCred(id, cred.ID)
	if sc == nil || sc.Surface == auth.Desktop {
		fail(w, 401, "auth_required", "Approve from your registered phone")
		return
	}
	sc.Cred.Authenticator, sc.Cred.Flags = cred.Authenticator, cred.Flags
	if req.Decision == "deny" {
		la.Denied = true
		writeJSON(w, 200, map[string]string{"state": "denied"})
		return
	}
	if err := la.Attempt.Pass(auth.F2, s.cfg.Now()); err != nil {
		fail(w, 401, "auth_required", "This sign-in has expired")
		return
	}
	la.Approved = true
	writeJSON(w, 200, map[string]string{"state": "approved"})
}

func (s *Server) handleSecondKeyBegin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LoginID string `json:"login_id"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, 422, "validation_failed", "Malformed request")
		return
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	la, ok := s.loginFrom(req.LoginID)
	if !ok || la.Attempt == nil || la.Attempt.Surface != auth.Desktop || la.Attempt.Complete() {
		fail(w, 400, "auth_required", "This sign-in has expired. Start again.")
		return
	}
	id := s.store.identities[la.IdentityID]
	var allowed []protocol.CredentialDescriptor
	for _, c := range id.Credentials {
		if !c.Revoked && c.Kind == KindRoamingKey && !bytes.Equal(c.Cred.ID, la.FirstCredID) {
			allowed = append(allowed, c.Cred.Descriptor())
		}
	}
	if len(allowed) == 0 {
		fail(w, 422, "validation_failed", "No second security key registered. Approve on your phone instead.")
		return
	}
	assertion, data, err := s.wa.BeginLogin(webauthnUser{id}, webauthn.WithAllowedCredentials(allowed), webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		fail(w, 500, "auth_required", "Could not start")
		return
	}
	la.Assertion = data
	writeJSON(w, 200, map[string]any{"options": assertion})
}

func (s *Server) handleSecondKeyFinish(w http.ResponseWriter, r *http.Request) {
	var env credentialEnvelope
	if err := decode(r, &env); err != nil {
		fail(w, 422, "validation_failed", "Malformed request")
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(env.Credential)
	if err != nil {
		genericLoginFailure(w)
		return
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	la, ok := s.loginFrom(env.LoginID)
	if !ok || la.Attempt == nil || la.Assertion == nil || la.Attempt.Surface != auth.Desktop {
		fail(w, 400, "auth_required", "This sign-in has expired. Start again.")
		return
	}
	data := la.Assertion
	la.Assertion = nil
	id := s.store.identities[la.IdentityID]
	cred, err := s.wa.ValidateLogin(webauthnUser{id}, *data, parsed)
	if err != nil || !cred.Flags.UserVerified || cred.Authenticator.CloneWarning || bytes.Equal(cred.ID, la.FirstCredID) {
		s.recordFailure(id)
		genericLoginFailure(w)
		return
	}
	sc := findCred(id, cred.ID)
	if sc == nil || sc.Kind != KindRoamingKey {
		genericLoginFailure(w)
		return
	}
	sc.Cred.Authenticator, sc.Cred.Flags = cred.Authenticator, cred.Flags
	if err := la.Attempt.Pass(auth.F2, s.cfg.Now()); err != nil {
		genericLoginFailure(w)
		return
	}
	s.finish(w, la)
}

// ---------------------------------------------------------------- session endpoints

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	sess, _, ok := s.sessionFrom(r)
	if !ok {
		fail(w, 401, "auth_required", "Sign in required")
		return
	}
	id := s.store.identities[sess.IdentityID]
	active := 0
	for _, c := range id.Credentials {
		if !c.Revoked {
			active++
		}
	}
	writeJSON(w, 200, map[string]any{
		"session_kind": sess.Kind, "surface": sess.Surface, "tier": id.Tier.String(),
		"passkeys": active, "expires_at": sess.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if _, th, ok := s.sessionFrom(r); ok {
		delete(s.store.sessions, th)
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleBurn signs the member out everywhere. It is never gated by step-up (02 §3.3); the
// client wipes its local data.
func (s *Server) handleBurn(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	sess, _, ok := s.sessionFrom(r)
	if !ok {
		w.WriteHeader(http.StatusNoContent) // uniform: nothing to reveal
		return
	}
	for th, other := range s.store.sessions {
		if other.IdentityID == sess.IdentityID {
			delete(s.store.sessions, th)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
