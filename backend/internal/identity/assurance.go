package identity

// Identity and age assurance (02 §3.1–3.2; FR-094, FR-097; D-39, D-41, D-61).
//
//   - V1: a liveness check (randomised challenge) at the provider, matched to the member's
//     profile photos. TRYST learns pass or fail.
//   - V2: a photo-ID check at the provider (document chip read where possible, liveness,
//     face match to the document). TRYST receives the provider's over-18 decision, the birth
//     year-month and the document number, re-checks the age itself, turns the document
//     number into a one-way code at once and discards it. It also needs the member's
//     conduct declaration (ToS 3.5).
//   - One person, one account: a document already holding an account is refused (recover
//     that account instead), and so is a document on the ban list.
//   - Someone under 18 is refused and locked out; nothing they uploaded is ever shown.
//
// The provider is called through AssuranceProvider. Dev uses a local stand-in that
// production refuses.

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/admission"
	"github.com/jnnseya-cpu/tryst/backend/internal/gate"
	"github.com/jnnseya-cpu/tryst/backend/internal/profile"
)

// ConductVersion is the conduct declaration members accept (ToS cl. 3.5).
const ConductVersion = "tos-3.5/2026-10"

// AssuranceSession is one hand-off to the provider.
type AssuranceSession struct {
	ID       string    `json:"id"`
	Account  string    `json:"account"`
	Kind     string    `json:"kind"`  // liveness | age
	State    string    `json:"state"` // pending | passed | failed | refused | duplicate | not_eligible
	Created  time.Time `json:"created"`
	Finished time.Time `json:"finished,omitempty"`
	Recovery string    `json:"recovery,omitempty"` // set when the check is part of a recovery
}

// AssuranceResult is what the provider reports.
type AssuranceResult struct {
	SessionID      string `json:"session_id"`
	Result         string `json:"result"` // pass | fail
	IsAdult        bool   `json:"is_adult"`
	BirthYM        string `json:"birth_year_month"`
	DocumentNumber string `json:"document_number"`
	Method         string `json:"method"`
}

// AssuranceProvider starts provider sessions and authenticates their callbacks.
type AssuranceProvider interface {
	Name() string
	Start(ctx context.Context, sess AssuranceSession, returnURL string) (url string, err error)
	VerifyCallback(r *http.Request, body []byte) error
}

// DevAssurance is the local stand-in: the member completes a form at /verify/dev that
// plays the provider. Refused outside dev.
type DevAssurance struct{}

func (DevAssurance) Name() string { return "dev" }
func (DevAssurance) Start(_ context.Context, sess AssuranceSession, _ string) (string, error) {
	return "/verify/dev?session=" + sess.ID + "&kind=" + sess.Kind, nil
}
func (DevAssurance) VerifyCallback(*http.Request, []byte) error {
	return errors.New("dev assurance has no callbacks")
}

// ErrAssuranceUnavailable means no certified provider can run the check now (G-NG-4).
var ErrAssuranceUnavailable = errors.New("verification unavailable")

type assuranceState struct {
	sessions map[string]*AssuranceSession
	index    *admission.Index
	bans     map[string]bool // document anchor (hex) -> banned
}

func (s *Server) loadAssurance(ctx context.Context) error {
	s.as = &assuranceState{sessions: map[string]*AssuranceSession{}, index: admission.NewIndex(), bans: map[string]bool{}}
	for id, i := range s.store.identities {
		if i.AnchorDoc != "" {
			if b, err := hex.DecodeString(i.AnchorDoc); err == nil {
				s.as.index.Register(id, admission.Anchors{Doc: b}, "")
			}
		}
	}
	if s.cfg.Persist == nil {
		return nil
	}
	docs, err := s.cfg.Persist.All(ctx, "assurance")
	if err != nil {
		return err
	}
	for _, b := range docs {
		var a AssuranceSession
		if json.Unmarshal(b, &a) == nil {
			s.as.sessions[a.ID] = &a
		}
	}
	bans, err := s.cfg.Persist.All(ctx, "ban")
	if err != nil {
		return err
	}
	for k := range bans {
		s.as.bans[k] = true
	}
	return nil
}

func (s *Server) saveAssurance(ctx context.Context, a *AssuranceSession) {
	if s.cfg.Persist == nil {
		return
	}
	b, _ := json.Marshal(a)
	if err := s.cfg.Persist.Put(ctx, "assurance", a.ID, b); err != nil {
		log.Printf("persist assurance failed: %v", err)
	}
}

// fullSessionQuiet is fullSession without writing an error. Caller holds the store lock.
func (s *Server) fullSessionQuiet(r *http.Request) *Identity {
	sess, _, ok := s.sessionFrom(r)
	if !ok || sess.Kind != Full {
		return nil
	}
	return s.store.get(sess.IdentityID)
}

// fullSession returns the caller's identity for a full session. Caller holds the store lock.
func (s *Server) fullSession(w http.ResponseWriter, r *http.Request) *Identity {
	sess, _, ok := s.sessionFrom(r)
	if !ok || sess.Kind != Full {
		fail(w, 401, "auth_required", "Please sign in again.")
		return nil
	}
	return s.store.get(sess.IdentityID)
}

func (s *Server) handleAssuranceStart(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind string `json:"kind"`
	}
	_ = decode(r, &in)
	s.store.mu.Lock()
	id := s.fullSession(w, r)
	if id == nil {
		s.store.mu.Unlock()
		return
	}
	switch {
	case in.Kind != "liveness" && in.Kind != "age":
		s.store.mu.Unlock()
		fail(w, 422, "validation_failed", "kind is liveness or age")
		return
	case in.Kind == "age" && id.Tier < gate.V1:
		s.store.mu.Unlock()
		fail(w, 409, "liveness_first", "Do the face check first.")
		return
	case s.cfg.Assurance == nil:
		s.store.mu.Unlock()
		fail(w, 503, "verification_unavailable", "Verification isn't available right now. Please try again later.")
		return
	}
	if s.cfg.AssuranceRoute != nil {
		if err := s.cfg.AssuranceRoute(); err != nil {
			s.store.mu.Unlock()
			fail(w, 503, "verification_unavailable", "Verification isn't available on this device right now. Please try again later.")
			return
		}
	}
	a := &AssuranceSession{ID: "as_" + randomHex(12), Account: id.ID(), Kind: in.Kind, State: "pending", Created: s.cfg.Now()}
	s.as.sessions[a.ID] = a
	s.store.mu.Unlock()
	s.saveAssurance(r.Context(), a)
	u, err := s.cfg.Assurance.Start(r.Context(), *a, s.cfg.AssuranceReturnURL)
	if err != nil {
		log.Printf("assurance start (%s): %v", s.cfg.Assurance.Name(), err)
		fail(w, 502, "verification_unavailable", "We couldn't start the check. Please try again.")
		return
	}
	writeJSON(w, 201, map[string]any{"session_id": a.ID, "url": u})
}

// handleAssuranceCallback receives the provider's signed result (sys).
func (s *Server) handleAssuranceCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil || s.cfg.Assurance == nil || s.cfg.Assurance.VerifyCallback(r, body) != nil {
		fail(w, 401, "bad_signature", "Bad signature")
		return
	}
	var res AssuranceResult
	if json.Unmarshal(body, &res) != nil {
		fail(w, 400, "bad_request", "Bad request")
		return
	}
	if _, err := s.applyAssurance(r.Context(), res); err != nil {
		fail(w, 404, "not_found", "Unknown session")
		return
	}
	w.WriteHeader(204)
}

// handleDevComplete plays the provider in dev only.
func (s *Server) handleDevComplete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.cfg.Assurance.(DevAssurance); !ok {
		fail(w, 404, "not_found", "Not found")
		return
	}
	var res AssuranceResult
	_ = decode(r, &res)
	s.store.mu.Lock()
	a := s.as.sessions[res.SessionID]
	var owner bool
	if a != nil && a.Recovery != "" {
		owner = subtle.ConstantTimeCompare([]byte(tokenHash(r.URL.Query().Get("recovery"))), []byte(a.Recovery)) == 1
	} else if id := s.fullSessionQuiet(r); id != nil && a != nil {
		owner = a.Account == id.ID()
	}
	s.store.mu.Unlock()
	if !owner {
		fail(w, 404, "not_found", "Not found")
		return
	}
	state, err := s.applyAssurance(r.Context(), res)
	if err != nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	writeJSON(w, 200, map[string]any{"state": state})
}

// applyAssurance records a provider result exactly once and returns the session state.
func (s *Server) applyAssurance(ctx context.Context, res AssuranceResult) (string, error) {
	s.store.mu.Lock()
	a := s.as.sessions[res.SessionID]
	if a == nil {
		s.store.mu.Unlock()
		return "", errors.New("unknown session")
	}
	if a.State != "pending" {
		st := a.State
		s.store.mu.Unlock()
		return st, nil
	}
	id := s.store.get(a.Account)
	now := s.cfg.Now()
	a.Finished = now
	switch {
	case a.Recovery != "":
		a.State = s.recoveryResult(a, id, res, now)
	case id == nil:
		a.State = "failed"
	case res.Result != "pass":
		a.State = "failed"
	case a.Kind == "liveness":
		a.State = "passed"
		id.LivenessAt = now
		if id.Tier < gate.V1 {
			id.Tier = gate.V1
		}
	default:
		a.State = s.ageResult(id, res, now)
	}
	state := a.State
	s.store.mu.Unlock()
	s.saveAssurance(ctx, a)
	if a.Recovery != "" {
		s.saveRecovery(ctx, a.Recovery)
	}
	s.flush(ctx)
	return state, nil
}

// ageResult applies a passed ID check. Caller holds the store lock.
func (s *Server) ageResult(id *Identity, res AssuranceResult, now time.Time) string {
	b, err := profile.ParseBirthYM(res.BirthYM)
	if err != nil || !profile.Adult(res.IsAdult, b, now) {
		id.Refused = "under_18"
		s.revokeAll(id.ID())
		id.record(now, "refused_under_18", "")
		return "refused"
	}
	docNumber := strings.TrimSpace(res.DocumentNumber)
	if docNumber == "" {
		return "failed"
	}
	anchors := admission.Derive(s.cfg.AnchorPepper, docNumber, nil, "")
	doc := hex.EncodeToString(anchors.Doc)
	if s.as.bans[doc] {
		id.Refused = "not_eligible"
		s.revokeAll(id.ID())
		return "not_eligible"
	}
	if id.AnchorDoc != "" && id.AnchorDoc != doc {
		return "document_mismatch" // re-verification must use the same document
	}
	if v := s.as.index.Register(id.ID(), anchors, ""); !v.Allowed {
		return "duplicate"
	}
	id.AnchorDoc, id.BirthYM, id.AgeCheckedAt = doc, b.String(), now
	s.promote(id)
	return "passed"
}

// promote raises the tier to V2 once the ID check and the conduct declaration are both in.
func (s *Server) promote(id *Identity) {
	if id.AnchorDoc != "" && !id.ConductAt.IsZero() && id.Tier < gate.V2 && id.Refused == "" {
		id.Tier = gate.V2
	}
}

// revokeAll ends every session of an account. Caller holds the store lock.
func (s *Server) revokeAll(account string) {
	for th, se := range s.store.sessions {
		if se.IdentityID == account {
			delete(s.store.sessions, th)
			s.store.sessDirty[th] = nil
		}
	}
}

func (s *Server) handleConduct(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version string `json:"version"`
		Accept  bool   `json:"accept"`
	}
	_ = decode(r, &in)
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	id := s.fullSession(w, r)
	if id == nil {
		return
	}
	if !in.Accept || in.Version != ConductVersion {
		fail(w, 422, "validation_failed", "Read and accept the current conduct declaration.")
		return
	}
	id.ConductAt, id.ConductVersion = s.cfg.Now(), in.Version
	if s.cfg.Consent != nil {
		s.cfg.Consent(id.ID(), "conduct_declaration", in.Version)
	}
	s.promote(id)
	writeJSON(w, 200, s.assuranceStatus(id))
}

func (s *Server) assuranceStatus(id *Identity) map[string]any {
	ageState := ""
	for _, a := range s.as.sessions {
		if a.Account == id.ID() && a.Kind == "age" && (ageState == "" || a.State != "pending") {
			ageState = a.State
		}
	}
	return map[string]any{"tier": id.Tier.String(), "liveness_done": !id.LivenessAt.IsZero(), "conduct_accepted": !id.ConductAt.IsZero(),
		"conduct_version": ConductVersion, "id_checked": id.AnchorDoc != "", "age_check": ageState, "refused": id.Refused}
}

func (s *Server) handleAssuranceStatus(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	id := s.fullSession(w, r)
	if id == nil {
		return
	}
	writeJSON(w, 200, s.assuranceStatus(id))
}

// BanMember copies a permanently removed member's document code to the ban list (ToS
// 15.3: the declared exception that survives erasure). Lift undoes it after an appeal.
func (s *Server) BanMember(ctx context.Context, account string, banned bool) {
	s.store.mu.Lock()
	id := s.store.identities[account]
	doc := ""
	if id != nil {
		doc = id.AnchorDoc
	}
	if doc != "" {
		if banned {
			s.as.bans[doc] = true
		} else {
			delete(s.as.bans, doc)
		}
	}
	s.store.mu.Unlock()
	if doc == "" || s.cfg.Persist == nil {
		return
	}
	var err error
	if banned {
		b, _ := json.Marshal(map[string]any{"at": s.cfg.Now()})
		err = s.cfg.Persist.Put(ctx, "ban", doc, b)
	} else {
		err = s.cfg.Persist.Delete(ctx, "ban", doc)
	}
	if err != nil {
		log.Printf("persist ban failed: %v", err)
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
