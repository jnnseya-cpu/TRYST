package identity

// Account recovery (02 §3.3; FR-061; D-62). A member who has lost every device proves
// they are the account holder again with the provider: a liveness check and an ID check
// on the same document the account was verified with (its one-way code must match). A
// 24-hour hold follows, during which any signed-in device can cancel; the member is told
// when recovery starts and when the hold begins. After the hold every old passkey and
// session is revoked and a short recovery session may register a new passkey.
//
// Responses never say whether a contact has an account: an unknown contact gets a
// recovery that can never complete. Staff cannot bypass any of this.

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/admission"
)

const (
	RecoveryHold        = 24 * time.Hour
	recoveryChecksTTL   = 24 * time.Hour     // the checks must be done within this
	recoveryReadyTTL    = 7 * 24 * time.Hour // finish within this after the hold
	recoverySessionTTL  = 15 * time.Minute
	recoveryMaxPerDay   = 3
	recoveryStateChecks = "checks"
	recoveryStateHold   = "hold"
)

// Recovery is one recovery attempt, stored under the hash of its ID (the ID is the
// bearer secret the recovering browser holds).
type Recovery struct {
	Hash       string    `json:"hash"`
	Account    string    `json:"account,omitempty"` // empty: unknown contact, never completes
	Started    time.Time `json:"started"`
	LivenessOK bool      `json:"liveness_ok,omitempty"`
	DocOK      bool      `json:"doc_ok,omitempty"`
	HoldUntil  time.Time `json:"hold_until,omitempty"`
	State      string    `json:"state"` // checks | hold | failed | cancelled | done
}

// state reports the effective state, expiring stale attempts.
func (rc *Recovery) state(now time.Time) string {
	switch {
	case rc.State == recoveryStateChecks && now.After(rc.Started.Add(recoveryChecksTTL)):
		return "expired"
	case rc.State == recoveryStateHold && now.After(rc.HoldUntil.Add(recoveryReadyTTL)):
		return "expired"
	case rc.State == recoveryStateHold && !now.Before(rc.HoldUntil):
		return "ready"
	}
	return rc.State
}

func (rc *Recovery) open(now time.Time) bool {
	st := rc.state(now)
	return st == recoveryStateChecks || st == recoveryStateHold || st == "ready"
}

type recoveryState struct {
	byHash    map[string]*Recovery
	starts    map[string][]time.Time // contact hash -> recent begins
	lockTries map[string][]time.Time // contact hash -> recent emergency-lock attempts
}

func (s *Server) loadRecovery(ctx context.Context) error {
	s.rec = &recoveryState{byHash: map[string]*Recovery{}, starts: map[string][]time.Time{}, lockTries: map[string][]time.Time{}}
	if s.cfg.Persist == nil {
		return nil
	}
	docs, err := s.cfg.Persist.All(ctx, "recovery")
	if err != nil {
		return err
	}
	for _, b := range docs {
		var rc Recovery
		if json.Unmarshal(b, &rc) == nil {
			s.rec.byHash[rc.Hash] = &rc
		}
	}
	return nil
}

func (s *Server) saveRecovery(ctx context.Context, hash string) {
	if s.cfg.Persist == nil {
		return
	}
	s.store.mu.Lock()
	rc := s.rec.byHash[hash]
	var b []byte
	if rc != nil {
		b, _ = json.Marshal(rc)
	}
	s.store.mu.Unlock()
	if b == nil {
		return
	}
	if err := s.cfg.Persist.Put(ctx, "recovery", hash, b); err != nil {
		log.Printf("persist recovery failed: %v", err)
	}
}

func (s *Server) notify(account, kind string) {
	if s.cfg.Notify != nil && account != "" {
		s.cfg.Notify(account, kind, "/account")
	}
}

// handleRecoveryBegin starts a recovery for a contact. The answer is the same whether or
// not the contact has an account.
func (s *Server) handleRecoveryBegin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Contact string `json:"contact"`
	}
	if err := decode(r, &in); err != nil || !ValidContact(NormaliseContact(in.Contact)) {
		fail(w, 422, "validation_failed", "Enter the email or phone number on your account")
		return
	}
	ch := contactHash(s.cfg.ContactPepper, in.Contact)
	now := s.cfg.Now()
	s.store.mu.Lock()
	recent := s.rec.starts[ch][:0]
	for _, t := range s.rec.starts[ch] {
		if now.Sub(t) < 24*time.Hour {
			recent = append(recent, t)
		}
	}
	if len(recent) >= recoveryMaxPerDay {
		s.rec.starts[ch] = recent
		s.store.mu.Unlock()
		w.Header().Set("Retry-After", "86400")
		fail(w, 429, "rate_limited", "Too many recovery attempts. Try again tomorrow.")
		return
	}
	s.rec.starts[ch] = append(recent, now)
	raw := "rc_" + randomToken(24)
	rc := &Recovery{Hash: tokenHash(raw), Started: now, State: recoveryStateChecks}
	if acct, ok := s.store.byContact[ch]; ok {
		if id := s.store.identities[acct]; id != nil && id.AnchorDoc != "" && id.Refused == "" {
			rc.Account = acct
			id.record(now, "recovery_started", "")
			s.store.dirty[acct] = true
		}
	}
	s.rec.byHash[rc.Hash] = rc
	s.store.mu.Unlock()
	s.saveRecovery(r.Context(), rc.Hash)
	s.notify(rc.Account, "recovery_started")
	writeJSON(w, http.StatusAccepted, map[string]any{"recovery_id": raw, "hold_hours": int(RecoveryHold.Hours())})
}

// recoveryFrom finds the recovery named in the path. Caller holds the store lock.
func (s *Server) recoveryFrom(r *http.Request) *Recovery {
	return s.rec.byHash[tokenHash(r.PathValue("id"))]
}

func (s *Server) recoveryView(rc *Recovery) map[string]any {
	now := s.cfg.Now()
	out := map[string]any{"state": rc.state(now), "liveness_done": rc.LivenessOK, "id_done": rc.DocOK}
	if !rc.HoldUntil.IsZero() {
		out["hold_until"] = rc.HoldUntil.UTC().Format(time.RFC3339)
	}
	return out
}

func (s *Server) handleRecoveryStatus(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	rc := s.recoveryFrom(r)
	if rc == nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	writeJSON(w, 200, s.recoveryView(rc))
}

// handleRecoveryCheck hands the recovering member to the provider for one of the checks.
func (s *Server) handleRecoveryCheck(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind string `json:"kind"`
	}
	_ = decode(r, &in)
	raw := r.PathValue("id")
	s.store.mu.Lock()
	rc := s.recoveryFrom(r)
	switch {
	case rc == nil:
		s.store.mu.Unlock()
		fail(w, 404, "not_found", "Not found")
		return
	case rc.state(s.cfg.Now()) != recoveryStateChecks:
		s.store.mu.Unlock()
		fail(w, 409, "recovery_closed", "This recovery can't take more checks. Start again if you need to.")
		return
	case in.Kind != "liveness" && in.Kind != "age":
		s.store.mu.Unlock()
		fail(w, 422, "validation_failed", "kind is liveness or age")
		return
	case in.Kind == "age" && !rc.LivenessOK:
		s.store.mu.Unlock()
		fail(w, 409, "liveness_first", "Do the face check first.")
		return
	case s.cfg.Assurance == nil:
		s.store.mu.Unlock()
		fail(w, http.StatusServiceUnavailable, "verification_unavailable", "Verification isn't available right now. Please try again later.")
		return
	}
	if s.cfg.AssuranceRoute != nil {
		if err := s.cfg.AssuranceRoute(); err != nil {
			s.store.mu.Unlock()
			fail(w, http.StatusServiceUnavailable, "verification_unavailable", "Verification isn't available on this device right now. Please try again later.")
			return
		}
	}
	a := &AssuranceSession{ID: "as_" + randomHex(12), Account: rc.Account, Kind: in.Kind, State: "pending", Created: s.cfg.Now(), Recovery: rc.Hash}
	s.as.sessions[a.ID] = a
	s.store.mu.Unlock()
	s.saveAssurance(r.Context(), a)
	u, err := s.cfg.Assurance.Start(r.Context(), *a, s.cfg.RecoveryReturnURL)
	if err != nil {
		log.Printf("assurance start (%s): %v", s.cfg.Assurance.Name(), err)
		fail(w, 502, "verification_unavailable", "We couldn't start the check. Please try again.")
		return
	}
	if _, dev := s.cfg.Assurance.(DevAssurance); dev {
		u += "&recovery=" + raw // the dev stand-in proves ownership with the recovery ID
	}
	writeJSON(w, 201, map[string]any{"session_id": a.ID, "url": u})
}

// recoveryResult applies a provider result to a recovery. It never changes the tier.
// Caller holds the store lock.
func (s *Server) recoveryResult(a *AssuranceSession, id *Identity, res AssuranceResult, now time.Time) string {
	rc := s.rec.byHash[a.Recovery]
	if rc == nil || id == nil || rc.Account != id.ID() || rc.state(now) != recoveryStateChecks || res.Result != "pass" {
		return "failed"
	}
	if a.Kind == "liveness" {
		rc.LivenessOK = true
		return "passed"
	}
	anchors := admission.Derive(s.cfg.AnchorPepper, res.DocumentNumber, nil, "")
	doc := hex.EncodeToString(anchors.Doc)
	if res.DocumentNumber == "" || subtle.ConstantTimeCompare([]byte(doc), []byte(id.AnchorDoc)) != 1 {
		rc.State = "failed" // one document only: no trying others
		id.record(now, "recovery_failed", "")
		return "document_mismatch"
	}
	rc.DocOK = true
	rc.State, rc.HoldUntil = recoveryStateHold, now.Add(RecoveryHold)
	id.record(now, "recovery_hold", "")
	if s.cfg.Notify != nil {
		// Called under the store lock like new_device; Notify must not call back here.
		s.cfg.Notify(id.ID(), "recovery_hold", "/account")
	}
	return "passed"
}

// handleRecoveryCancel cancels every open recovery of the caller's account. Any signed-in
// device can do it; it never needs step-up.
func (s *Server) handleRecoveryCancel(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	id := s.fullSession(w, r)
	if id == nil {
		s.store.mu.Unlock()
		return
	}
	now := s.cfg.Now()
	var hashes []string
	for h, rc := range s.rec.byHash {
		if rc.Account == id.ID() && rc.open(now) {
			rc.State = "cancelled"
			hashes = append(hashes, h)
		}
	}
	if len(hashes) > 0 {
		s.store.get(id.ID()).record(now, "recovery_cancelled", "")
	}
	s.store.mu.Unlock()
	for _, h := range hashes {
		s.saveRecovery(r.Context(), h)
	}
	writeJSON(w, 200, map[string]any{"cancelled": len(hashes)})
}

// handleRecoveryFinish completes a recovery whose hold has passed: every old passkey and
// session is revoked and a short recovery session is issued to register a new passkey.
func (s *Server) handleRecoveryFinish(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	rc := s.recoveryFrom(r)
	if rc == nil {
		s.store.mu.Unlock()
		fail(w, 404, "not_found", "Not found")
		return
	}
	now := s.cfg.Now()
	st := rc.state(now)
	if st == recoveryStateHold {
		s.store.mu.Unlock()
		fail(w, 409, "recovery_hold", "Your account will be ready after the hold.")
		return
	}
	id := s.store.get(rc.Account)
	if st != "ready" || id == nil || id.Refused != "" {
		s.store.mu.Unlock()
		fail(w, 409, "recovery_closed", "This recovery can't be completed. Start again if you need to.")
		return
	}
	for _, c := range id.Credentials {
		c.Revoked = true
	}
	s.revokeAll(id.ID())
	id.Failures, id.LockedUntil = 0, time.Time{}
	unlocked := id.Locked
	id.Locked = false
	code := s.issueEmergencyCode(id) // the old one may be spent or known to someone else
	id.record(now, "recovered", "")
	rc.State = "done"
	tok := randomToken(32)
	sess := &Session{IdentityID: id.ID(), Kind: RecoverySession, LastSeen: now, ExpiresAt: now.Add(recoverySessionTTL)}
	s.store.sessions[tokenHash(tok)] = sess
	s.store.sessDirty[tokenHash(tok)] = sess
	s.store.mu.Unlock()
	s.saveRecovery(r.Context(), rc.Hash)
	if unlocked && s.cfg.OnLock != nil {
		s.cfg.OnLock(id.ID(), false)
	}
	writeJSON(w, 200, map[string]any{"access_token": tok, "expires_at": sess.ExpiresAt.UTC().Format(time.RFC3339), "kind": RecoverySession, "emergency_code": code})
}

// recoveryPending reports an open recovery on the account. Caller holds the store lock.
func (s *Server) recoveryPending(account string) map[string]any {
	now := s.cfg.Now()
	for _, rc := range s.rec.byHash {
		if rc.Account == account && rc.open(now) {
			return s.recoveryView(rc)
		}
	}
	return nil
}
