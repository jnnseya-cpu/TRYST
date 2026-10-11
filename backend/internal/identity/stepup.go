package identity

// Step-up and devices (02 §3.3; FR-060, FR-125; D-62). Step-up repeats both sign-in checks
// for the surface inside an existing session; sensitive actions are allowed for 5 minutes
// afterwards: adding or revoking a device, viewing the device list, exporting data,
// broadening discretion, couple changes, and the like. It is never needed for Burn, Panic,
// reporting, blocking, cancelling a subscription or deleting the account.

import (
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// StepUpWindow is how long a step-up lasts.
const StepUpWindow = 5 * time.Minute

func (s *Server) stepUpOK(sess *Session) bool {
	return sess != nil && s.cfg.Now().Before(sess.StepUpUntil)
}

// StepUpOK reports whether the request's session has a fresh step-up (other services).
func (s *Server) StepUpOK(r *http.Request) bool {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	sess, _, ok := s.sessionFrom(r)
	return ok && sess.Kind == Full && s.stepUpOK(sess)
}

// handleStepUpBegin starts the same two-factor ceremony as sign-in, bound to the caller's
// session and identity. The client continues with /v1/auth/login/passkey and the second
// factor; finishing marks this session stepped up instead of issuing a new one.
func (s *Server) handleStepUpBegin(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	sess, th, ok := s.sessionFrom(r)
	if !ok || sess.Kind != Full {
		s.store.mu.Unlock()
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	id := s.store.identities[sess.IdentityID]
	s.store.mu.Unlock()
	assertion, data, err := s.wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil || id == nil {
		fail(w, 500, "auth_required", "Could not start the check")
		return
	}
	loginID := randomToken(24)
	s.store.mu.Lock()
	s.store.logins[tokenHash(loginID)] = &LoginAttempt{Assertion: data, ExpiresAt: s.cfg.Now().Add(loginTTL), StepUpFor: th, ExpectID: id.ID()}
	s.store.mu.Unlock()
	writeJSON(w, 200, map[string]any{"login_id": loginID, "options": assertion})
}

// finishStepUp completes a step-up. Caller holds the store lock.
func (s *Server) finishStepUp(w http.ResponseWriter, la *LoginAttempt) {
	sess, ok := s.store.sessions[la.StepUpFor]
	if !ok || la.IdentityID != la.ExpectID || sess.IdentityID != la.ExpectID {
		genericLoginFailure(w)
		return
	}
	sess.StepUpUntil = s.cfg.Now().Add(StepUpWindow)
	s.store.sessDirty[la.StepUpFor] = sess
	writeJSON(w, 200, map[string]any{"step_up": true, "until": sess.StepUpUntil.UTC().Format(time.RFC3339)})
}

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	sess, _, ok := s.sessionFrom(r)
	if !ok || sess.Kind != Full {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	if !s.stepUpOK(sess) {
		fail(w, 403, "step_up_required", "Confirm it's you to see your devices.")
		return
	}
	id := s.store.identities[sess.IdentityID]
	type dev struct {
		ID      string    `json:"id"`
		Surface string    `json:"surface"`
		Kind    string    `json:"kind"`
		Added   time.Time `json:"added"`
		Revoked bool      `json:"revoked"`
	}
	out := []dev{}
	for _, c := range id.Credentials {
		out = append(out, dev{hex.EncodeToString(c.Cred.ID[:min(8, len(c.Cred.ID))]), string(c.Surface), c.Kind, c.CreatedAt, c.Revoked})
	}
	writeJSON(w, 200, map[string]any{"devices": out})
}

func (s *Server) handleDeviceRevoke(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	sess, _, ok := s.sessionFrom(r)
	if !ok || sess.Kind != Full {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	if !s.stepUpOK(sess) {
		fail(w, 403, "step_up_required", "Confirm it's you to remove a device.")
		return
	}
	id := s.store.get(sess.IdentityID)
	want := strings.ToLower(r.PathValue("id"))
	active := 0
	var target *StoredCredential
	for _, c := range id.Credentials {
		if !c.Revoked {
			active++
		}
		if hex.EncodeToString(c.Cred.ID[:min(8, len(c.Cred.ID))]) == want {
			target = c
		}
	}
	switch {
	case target == nil || target.Revoked:
		fail(w, 404, "not_found", "Not found")
	case active <= 1:
		fail(w, 409, "last_device", "You can't remove your only device. Add another first, or use Burn.")
	default:
		target.Revoked = true
		id.record(s.cfg.Now(), "passkey_revoked", string(target.Surface))
		w.WriteHeader(204)
	}
}
