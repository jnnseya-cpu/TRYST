package identity

// Emergency lock and partner-shield keys (02 §5.2; FR-064, FR-106, FR-127; D-45, D-64).
//
// Emergency lock: at sign-up each member gets a one-time emergency code. With only their
// contact and that code, and no sign-in, they can lock the account from any browser:
// every session ends, sign-in is refused, and the profile and its photos are hidden.
// Only a full recovery unlocks it. Answers are the same whether or not anything matched,
// so the endpoint can't be used to find accounts; misuse can only lock, never expose.
//
// Partner shield: the member's device hashes a partner's contact (SHA-256 of the
// normalised form) and the server keys it with the contact pepper at once. Every account
// carries the same keyed value for its own contact, made at sign-up, so a match needs no
// contact in clear anywhere.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	emergencyMaxPerDay = 5
	emergencyAlphabet  = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O, 1/I
)

// newEmergencyCode returns a code like "K7QM-2XRP-H9TD".
func newEmergencyCode() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	var out strings.Builder
	for i := 0; i < 12; i++ {
		if i > 0 && i%4 == 0 {
			out.WriteByte('-')
		}
		out.WriteByte(emergencyAlphabet[int(b[i])%len(emergencyAlphabet)]) // 256 % 32 == 0: unbiased
	}
	return out.String()
}

func normaliseCode(c string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(c)))
}

// emergencyHash keys the code to the account so equal codes on two accounts differ.
func (s *Server) emergencyHash(account, code string) string {
	m := hmac.New(sha256.New, s.cfg.ContactPepper)
	m.Write([]byte("emergency|" + account + "|" + normaliseCode(code)))
	return hex.EncodeToString(m.Sum(nil))
}

// issueEmergencyCode gives the account a new code. Caller holds the store lock.
func (s *Server) issueEmergencyCode(id *Identity) string {
	code := newEmergencyCode()
	id.EmergencyHash = s.emergencyHash(id.ID(), code)
	return code
}

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// BlindShield keys a device-hashed contact (lowercase hex SHA-256 of the normalised
// contact). It returns "" for anything else.
func (s *Server) BlindShield(digest string) string {
	digest = strings.ToLower(strings.TrimSpace(digest))
	if !sha256Hex.MatchString(digest) {
		return ""
	}
	m := hmac.New(sha256.New, s.cfg.ContactPepper)
	m.Write([]byte("shield|" + digest))
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Server) shieldKeyForContact(contact string) string {
	d := sha256.Sum256([]byte(NormaliseContact(contact)))
	return s.BlindShield(hex.EncodeToString(d[:]))
}

// ShieldKeyOf returns the account's own shield key ("" if it has none).
func (s *Server) ShieldKeyOf(account string) string {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if id := s.store.identities[account]; id != nil {
		return id.ShieldKey
	}
	return ""
}

// handleEmergencyLock locks an account with its contact and emergency code. No session.
func (s *Server) handleEmergencyLock(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Contact string `json:"contact"`
		Code    string `json:"code"`
	}
	if err := decode(r, &in); err != nil || !ValidContact(NormaliseContact(in.Contact)) || normaliseCode(in.Code) == "" {
		fail(w, 422, "validation_failed", "Enter the email or phone number on the account and the emergency code.")
		return
	}
	ch := contactHash(s.cfg.ContactPepper, in.Contact)
	now := s.cfg.Now()
	s.store.mu.Lock()
	recent := s.rec.lockTries[ch][:0]
	for _, t := range s.rec.lockTries[ch] {
		if now.Sub(t) < 24*time.Hour {
			recent = append(recent, t)
		}
	}
	if len(recent) >= emergencyMaxPerDay {
		s.rec.lockTries[ch] = recent
		s.store.mu.Unlock()
		w.Header().Set("Retry-After", "86400")
		fail(w, 429, "rate_limited", "Too many attempts. Try again tomorrow.")
		return
	}
	s.rec.lockTries[ch] = append(recent, now)
	locked := ""
	if acct, ok := s.store.byContact[ch]; ok {
		id := s.store.get(acct)
		if id != nil && id.EmergencyHash != "" &&
			subtle.ConstantTimeCompare([]byte(id.EmergencyHash), []byte(s.emergencyHash(acct, in.Code))) == 1 {
			id.Locked, id.EmergencyHash = true, "" // single use; a new one comes with recovery
			s.revokeAll(acct)
			id.record(now, "emergency_lock", "")
			locked = acct
		}
	}
	s.store.mu.Unlock()
	if locked != "" && s.cfg.OnLock != nil {
		s.cfg.OnLock(locked, true)
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "received"})
}

// handleEmergencyCode issues a new emergency code (step-up), replacing the old one.
func (s *Server) handleEmergencyCode(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	sess, _, ok := s.sessionFrom(r)
	if !ok || sess.Kind != Full {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	if !s.stepUpOK(sess) {
		fail(w, 403, "step_up_required", "Confirm it's you to get a new emergency code.")
		return
	}
	id := s.store.get(sess.IdentityID)
	code := s.issueEmergencyCode(id)
	id.record(s.cfg.Now(), "emergency_code_issued", "")
	writeJSON(w, 201, map[string]string{"emergency_code": code})
}
