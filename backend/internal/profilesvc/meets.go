package profilesvc

// Meets: plan, door check, check-ins, duress and Panic (FR-020–FR-022, FR-099, FR-100, FR-115).
//
// The venue never reaches TRYST: it travels inside the encrypted thread together with a
// random door secret, from which both phones derive the same three door-check words. The
// server keeps only the meet's time window and state, and runs the check-in timer so a missed
// check-in raises the alarm even if the member's phone is switched off or taken.
//
// Check-ins use the member's check-in PIN; a separate duress PIN produces exactly the same
// response but starts escalation at once. Escalation goes to the trusted contact the member
// chose for this meet (deleted when it ends), then to the safety line (meet.Line) if they
// opted in. Panic escalates immediately.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/meet"
)

type meetRecord struct {
	ID        string    `json:"id"`
	Thread    string    `json:"thread"`
	Proposer  string    `json:"proposer"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	State     string    `json:"state"` // proposed | confirmed | cancelled | done
	Confirmed time.Time `json:"confirmed,omitempty"`
}

// safetySession is one member's live safety plan for a meet. Contact details live only here
// and only until the meet ends.
type safetySession struct {
	Meet       string
	Member     string
	Alias      string
	Trusted    string // phone or email, for this meet only
	SafetyLine bool
	sess       *meet.Session
	salt       []byte
	cases      []*meet.Case
	Ended      bool
	Started    time.Time
}

// Alerting delivers escalations. Production: SMS/email to the trusted contact and the
// alarm-receiving centre's API; dev: the log and the safety console queue.
type Alerting interface {
	TrustedContact(ctx context.Context, to, alias, window string, duress bool) error
	SafetyLine(ctx context.Context, a meet.Alert) error
	Emergency(ctx context.Context, a meet.Alert) error
}

// LogAlerting records alerts in memory and the log (dev and tests).
type LogAlerting struct {
	Trusted []string
	Line    []meet.Alert
	Panics  []meet.Alert
}

func (l *LogAlerting) TrustedContact(_ context.Context, to, alias, window string, duress bool) error {
	l.Trusted = append(l.Trusted, to)
	log.Printf("ALERT trusted contact (dev sink): alias=%s window=%s duress=%v", alias, window, duress)
	return nil
}
func (l *LogAlerting) SafetyLine(_ context.Context, a meet.Alert) error {
	l.Line = append(l.Line, a)
	log.Printf("ALERT safety line (dev sink): %s %s", a.ID, a.Reason)
	return nil
}
func (l *LogAlerting) Emergency(_ context.Context, a meet.Alert) error {
	l.Panics = append(l.Panics, a)
	log.Printf("ALERT emergency (dev sink): %s", a.ID)
	return nil
}

type meetState struct {
	meets    map[string]*meetRecord
	sessions map[string]*safetySession // meet|member
}

func newMeetState() *meetState {
	return &meetState{meets: map[string]*meetRecord{}, sessions: map[string]*safetySession{}}
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Service) saveMeet(ctx context.Context, m *meetRecord) {
	b, _ := json.Marshal(m)
	if err := s.st.Put(ctx, "meet", m.ID, b); err != nil {
		log.Printf("persist meet failed: %v", err)
	}
}

func (s *Service) loadMeets(ctx context.Context) error {
	docs, err := s.st.All(ctx, "meet")
	if err != nil {
		return err
	}
	for _, b := range docs {
		var m meetRecord
		if json.Unmarshal(b, &m) == nil {
			s.mt.meets[m.ID] = &m
		}
	}
	return nil
}

func (s *Service) meetFor(w http.ResponseWriter, me, id string) (*meetRecord, *thread) {
	m, ok := s.mt.meets[id]
	if !ok {
		fail(w, 404, "not_found", "Not found")
		return nil, nil
	}
	t, ok := s.th.threads[m.Thread]
	if !ok || !t.has(me) {
		fail(w, 404, "not_found", "Not found")
		return nil, nil
	}
	return m, t
}

// POST /v1/threads/{id}/meets {start, end} -> meet id. The place is not sent to the server.
func (s *Service) handleProposeMeet(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	t := s.threadFor(w, acc.ID, r.PathValue("id"))
	if t == nil {
		return
	}
	if t.State != "normal" || s.isBlocked(t.Members[0], t.Members[1]) {
		fail(w, 409, "thread_closed", "This conversation is closed.")
		return
	}
	var in struct {
		Start time.Time `json:"start"`
		End   time.Time `json:"end"`
	}
	now := s.now()
	if json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in) != nil || !in.End.After(in.Start) ||
		in.End.Sub(in.Start) > 12*time.Hour || in.Start.Before(now.Add(-time.Hour)) || in.Start.After(now.AddDate(0, 1, 0)) {
		fail(w, 422, "validation_failed", "Choose a time within the next month, up to 12 hours long.")
		return
	}
	m := &meetRecord{ID: "mt_" + randHex(10), Thread: t.ID, Proposer: acc.ID, Start: in.Start.UTC(), End: in.End.UTC(), State: "proposed"}
	s.mt.meets[m.ID] = m
	s.saveMeet(r.Context(), m)
	s.notify(t.other(acc.ID), "meet_proposed", "/messages/"+t.ID+"/meet")
	writeJSON(w, 201, s.meetView(acc.ID, m))
}

func (s *Service) meetView(me string, m *meetRecord) map[string]any {
	v := map[string]any{"id": m.ID, "thread": m.Thread, "start": m.Start, "end": m.End, "state": m.State, "mine": m.Proposer == me}
	if ss, ok := s.mt.sessions[m.ID+"|"+me]; ok && !ss.Ended {
		v["safety_active"] = true
		v["safety_line"] = ss.SafetyLine
	}
	return v
}

func (s *Service) handleGetMeet(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	m, _ := s.meetFor(w, acc.ID, r.PathValue("meet"))
	if m == nil {
		return
	}
	writeJSON(w, 200, s.meetView(acc.ID, m))
}

// POST /v1/meets/{meet}/confirm | /cancel
func (s *Service) handleMeetDecision(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	m, _ := s.meetFor(w, acc.ID, r.PathValue("meet"))
	if m == nil {
		return
	}
	switch r.PathValue("decision") {
	case "confirm":
		if m.Proposer == acc.ID || m.State != "proposed" {
			fail(w, 409, "not_allowed", "Only the other person can confirm a proposed meet.")
			return
		}
		m.State, m.Confirmed = "confirmed", s.now()
	case "cancel":
		if m.State == "done" {
			fail(w, 409, "not_allowed", "This meet has already happened.")
			return
		}
		m.State = "cancelled"
	default:
		fail(w, 404, "not_found", "Not found")
		return
	}
	s.saveMeet(r.Context(), m)
	if t := s.th.threads[m.Thread]; t != nil {
		s.notify(t.other(acc.ID), "meet_update", "/messages/"+t.ID+"/meet")
	}
	writeJSON(w, 200, s.meetView(acc.ID, m))
}

// POST /v1/meets/{meet}/safety: start the member's check-ins for this meet.
func (s *Service) handleSafetyStart(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	m, _ := s.meetFor(w, acc.ID, r.PathValue("meet"))
	if m == nil {
		return
	}
	if m.State != "confirmed" {
		fail(w, 409, "not_confirmed", "Start check-ins once the meet is confirmed.")
		return
	}
	var in struct {
		EveryMinutes int    `json:"every_minutes"`
		PIN          string `json:"pin"`
		DuressPIN    string `json:"duress_pin"`
		Trusted      string `json:"trusted_contact"`
		SafetyLine   bool   `json:"safety_line"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in) != nil {
		fail(w, 422, "validation_failed", "Check the details and try again.")
		return
	}
	if len(in.PIN) < 4 || len(in.DuressPIN) < 4 {
		fail(w, 422, "validation_failed", "PINs need at least 4 digits.")
		return
	}
	in.Trusted = strings.TrimSpace(in.Trusted)
	if in.Trusted != "" && !(strings.Contains(in.Trusted, "@") || strings.HasPrefix(in.Trusted, "+")) {
		fail(w, 422, "validation_failed", "Give your trusted contact as an email or a phone number starting with +.")
		return
	}
	salt := []byte(randHex(16))
	plan, err := meet.NewPlan(m.Start, m.End, time.Duration(in.EveryMinutes)*time.Minute, in.PIN, in.DuressPIN, salt, in.Trusted != "", in.SafetyLine)
	if err != nil {
		fail(w, 422, "validation_failed", sentencePlan(err))
		return
	}
	start := s.now()
	if m.Start.After(start) {
		plan.Start = m.Start
	} else {
		plan.Start = start
	}
	alias := p.DisplayName
	ss := &safetySession{Meet: m.ID, Member: acc.ID, Alias: alias, Trusted: in.Trusted, SafetyLine: in.SafetyLine,
		sess: meet.Start(plan, salt), salt: salt, Started: start}
	s.mt.sessions[m.ID+"|"+acc.ID] = ss
	writeJSON(w, 200, s.meetView(acc.ID, m))
}

func sentencePlan(err error) string {
	switch err {
	case meet.ErrPinsMustDiffer:
		return "Your duress PIN must be different from your check-in PIN."
	case meet.ErrNoEscalation:
		return "Add a trusted contact or turn on the safety line, so someone is told if you don't check in."
	case meet.ErrInterval:
		return "Choose check-ins between every 15 minutes and every 3 hours."
	}
	return "Check the details and try again."
}

// POST /v1/meets/{meet}/checkin {pin}: the reply is identical for the check-in and duress PIN.
func (s *Service) handleCheckIn(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	ss, ok := s.mt.sessions[r.PathValue("meet")+"|"+acc.ID]
	if !ok || ss.Ended {
		fail(w, 404, "not_found", "No check-ins running for this meet.")
		return
	}
	var in struct {
		PIN string `json:"pin"`
		End bool   `json:"end"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in) != nil {
		fail(w, 422, "validation_failed", "Enter your PIN.")
		return
	}
	now := s.now()
	if in.End {
		if ss.sess.End(in.PIN) {
			s.endSession(ss)
			writeJSON(w, 200, map[string]any{"message": "Glad you're safe. Check-ins have stopped.", "ended": true})
			return
		}
		// A duress PIN at "end" must not end anything, and must look like a normal reply.
		msg := ss.sess.CheckIn(in.PIN, now)
		if msg != "Thanks, checked in." {
			fail(w, 422, "pin_mismatch", msg)
			return
		}
		s.tickSession(r.Context(), ss, now)
		writeJSON(w, 200, map[string]any{"message": "Glad you're safe. Check-ins have stopped.", "ended": true})
		return
	}
	msg := ss.sess.CheckIn(in.PIN, now)
	if msg != "Thanks, checked in." {
		fail(w, 422, "pin_mismatch", msg)
		return
	}
	s.tickSession(r.Context(), ss, now)
	writeJSON(w, 200, map[string]any{"message": msg})
}

func (s *Service) endSession(ss *safetySession) {
	ss.Ended = true
	ss.Trusted = "" // contact details deleted when the meet ends
}

// POST /v1/safety/panic {meet_id?, lat?, lon?}: emergency. Exact location is accepted here
// and only here (P7, 02 §1).
func (s *Service) handlePanic(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		Meet string   `json:"meet_id"`
		Lat  *float64 `json:"lat"`
		Lon  *float64 `json:"lon"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in)
	loc := "not shared"
	if in.Lat != nil && in.Lon != nil {
		loc = strings.TrimRight(strings.TrimRight(jsonNum(*in.Lat), "0"), ".") + "," + strings.TrimRight(strings.TrimRight(jsonNum(*in.Lon), "0"), ".")
	}
	a := meet.Alert{ID: "al_" + randHex(8), Reason: "panic", MemberAlias: p.DisplayName, Location: loc, Started: s.now()}
	if ss, ok := s.mt.sessions[in.Meet+"|"+acc.ID]; ok && !ss.Ended {
		ss.sess.Panic()
		s.tickSession(r.Context(), ss, s.now())
	}
	if s.Alerts != nil {
		_ = s.Alerts.Emergency(r.Context(), a)
	}
	writeJSON(w, 200, map[string]any{"alert": a.ID, "call": "999", "silent_solution": "If you can't speak, dial 999, then press 55 when asked."})
}

func jsonNum(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// tickSession turns due actions into alerts. Caller holds s.mu.
func (s *Service) tickSession(ctx context.Context, ss *safetySession, now time.Time) {
	if ss.Ended {
		return
	}
	window := ""
	if m := s.mt.meets[ss.Meet]; m != nil {
		window = m.Start.Format("15:04") + "–" + m.End.Format("15:04")
	}
	for _, act := range ss.sess.Tick(now) {
		reason := "missed_check_in"
		duress := false
		if act == meet.Emergency {
			reason = "panic"
		}
		a := meet.Alert{ID: "al_" + randHex(8), Reason: reason, MemberAlias: ss.Alias, MeetWindow: window, Started: now}
		switch act {
		case meet.AlertTrusted:
			if s.Alerts != nil && ss.Trusted != "" {
				_ = s.Alerts.TrustedContact(ctx, ss.Trusted, ss.Alias, window, duress)
			}
		case meet.AlertSafetyLine:
			if s.Alerts != nil {
				_ = s.Alerts.SafetyLine(ctx, a)
			}
		case meet.Emergency:
			if s.Alerts != nil {
				_ = s.Alerts.Emergency(ctx, a)
			}
		}
	}
}

// SweepSafety runs every check-in timer; call it every 30 seconds. A session keeps running
// until the member ends it with their check-in PIN, so a member who never gets home safe
// keeps being looked for.
func (s *Service) SweepSafety(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for _, ss := range s.mt.sessions {
		s.tickSession(ctx, ss, now)
	}
}
