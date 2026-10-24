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
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/jnnseya-cpu/tryst/backend/internal/meet"
	"github.com/jnnseya-cpu/tryst/backend/internal/metrics"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
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

// dispatch delivers an alert outside the service lock (the alarm-receiving centre may be
// slow), retrying once; failures are counted for the on-call alert rule (deploy/monitoring).
func (s *Service) dispatch(channel string, send func(ctx context.Context) error) {
	if s.Alerts == nil {
		metrics.Default.Inc("tryst_safety_alert_failures_total", "channel", channel, "cause", "not_configured")
		return
	}
	s.alertWG.Add(1)
	go func() {
		defer s.alertWG.Done()
		for attempt := 0; attempt < 2; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			err := send(ctx)
			cancel()
			if err == nil {
				metrics.Default.Inc("tryst_safety_alerts_total", "channel", channel)
				return
			}
			log.Printf("safety alert (%s) failed: %v", channel, err)
			if attempt == 0 {
				time.Sleep(s.alertRetry)
			}
		}
		metrics.Default.Inc("tryst_safety_alert_failures_total", "channel", channel, "cause", "delivery")
	}()
}

// WaitAlerts blocks until dispatched alerts finish (tests, shutdown).
func (s *Service) WaitAlerts() { s.alertWG.Wait() }

// LogAlerting records alerts in memory and the log (dev and tests).
type LogAlerting struct {
	mu      sync.Mutex
	Trusted []string
	Urgent  []bool // per trusted-contact alert: sent with the duress wording
	Line    []meet.Alert
	Panics  []meet.Alert
}

func (l *LogAlerting) TrustedContact(_ context.Context, to, alias, window string, duress bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Trusted = append(l.Trusted, to)
	l.Urgent = append(l.Urgent, duress)
	log.Printf("ALERT trusted contact (dev sink): alias=%s window=%s duress=%v", alias, window, duress)
	return nil
}
func (l *LogAlerting) SafetyLine(_ context.Context, a meet.Alert) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Line = append(l.Line, a)
	log.Printf("ALERT safety line (dev sink): %s %s", a.ID, a.Reason)
	return nil
}
func (l *LogAlerting) Emergency(_ context.Context, a meet.Alert) error {
	l.mu.Lock()
	defer l.mu.Unlock()
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
	return s.loadSafety(ctx)
}

// savedSafety is a running check-in plan as stored, so a restart or deploy never silently
// drops someone's safety net. The trusted contact is sealed under the member's own key.
type savedSafety struct {
	Meet, Member, Alias string
	SafetyLine          bool
	Started             time.Time
	Trusted             []byte `json:",omitempty"`
	State               meet.State
}

func safetyKey(ss *safetySession) string { return ss.Meet + "|" + ss.Member }

// saveSafety stores a running plan, or deletes it once it has ended. Caller holds s.mu.
func (s *Service) saveSafety(ctx context.Context, ss *safetySession) {
	if ss.Ended {
		if err := s.st.Delete(ctx, "safety", safetyKey(ss)); err != nil && !errors.Is(err, store.ErrNotFound) {
			log.Printf("persist safety (end) failed: %v", err)
		}
		return
	}
	doc := savedSafety{Meet: ss.Meet, Member: ss.Member, Alias: ss.Alias, SafetyLine: ss.SafetyLine, Started: ss.Started, State: ss.sess.State()}
	if ss.Trusted != "" {
		sealed, err := s.vault.Seal(ctx, ss.Member, []byte(ss.Trusted), []byte("safety:"+safetyKey(ss)))
		if err != nil {
			log.Printf("persist safety failed (seal): %v", err)
			metrics.Default.Inc("tryst_safety_persist_failures_total")
			return
		}
		doc.Trusted = sealed
	}
	b, _ := json.Marshal(doc)
	if err := s.st.Put(ctx, "safety", safetyKey(ss), b); err != nil {
		log.Printf("persist safety failed: %v", err)
		metrics.Default.Inc("tryst_safety_persist_failures_total")
	}
}

func (s *Service) loadSafety(ctx context.Context) error {
	docs, err := s.st.All(ctx, "safety")
	if err != nil {
		return err
	}
	for k, b := range docs {
		var d savedSafety
		if json.Unmarshal(b, &d) != nil {
			continue
		}
		ss := &safetySession{Meet: d.Meet, Member: d.Member, Alias: d.Alias, SafetyLine: d.SafetyLine, Started: d.Started,
			sess: meet.Resume(d.State), salt: d.State.Salt}
		if len(d.Trusted) > 0 {
			pt, err := s.vault.Open(ctx, d.Member, d.Trusted, []byte("safety:"+k))
			if err != nil {
				// The member's key is gone (erased) or the record is damaged: nothing to resume.
				log.Printf("safety plan %s not resumed: %v", k, err)
				continue
			}
			ss.Trusted = string(pt)
		}
		s.mt.sessions[k] = ss
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
	if in.Trusted != "" && !strings.Contains(in.Trusted, "@") && s.EmailOnlyAlerts {
		// No SMS channel: an alert to a phone number would go nowhere.
		fail(w, 422, "email_only", "Give your trusted contact's email address. Alerts by text message aren't available yet.")
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
	s.saveSafety(r.Context(), ss)
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
			s.saveSafety(r.Context(), ss)
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
		s.saveSafety(r.Context(), ss)
		writeJSON(w, 200, map[string]any{"message": "Glad you're safe. Check-ins have stopped.", "ended": true})
		return
	}
	msg := ss.sess.CheckIn(in.PIN, now)
	if msg != "Thanks, checked in." {
		fail(w, 422, "pin_mismatch", msg)
		return
	}
	s.tickSession(r.Context(), ss, now)
	s.saveSafety(r.Context(), ss)
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
		s.saveSafety(r.Context(), ss)
	}
	s.dispatch("emergency", func(ctx context.Context) error { return s.Alerts.Emergency(ctx, a) })
	writeJSON(w, 200, map[string]any{"alert": a.ID, "call": "999", "silent_solution": "If you can't speak, dial 999, then press 55 when asked."})
}

func jsonNum(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// ukTime is the clock members and responders read; meets are stored in UTC. The zone data
// is compiled into the binary (time/tzdata), so the slim runtime image needs none.
var ukTime = func() *time.Location {
	l, err := time.LoadLocation("Europe/London")
	if err != nil {
		panic(err)
	}
	return l
}()

// meetWindow is the meet's time window on the UK clock, labelled so nobody reads it as UTC.
func meetWindow(start, end time.Time) string {
	return start.In(ukTime).Format("15:04") + "–" + end.In(ukTime).Format("15:04") + " UK time"
}

// tickSession turns due actions into alerts. Caller holds s.mu.
func (s *Service) tickSession(ctx context.Context, ss *safetySession, now time.Time) {
	if ss.Ended {
		return
	}
	window := ""
	if m := s.mt.meets[ss.Meet]; m != nil {
		window = meetWindow(m.Start, m.End)
	}
	acts := ss.sess.Tick(now)
	if len(acts) > 0 {
		defer s.saveSafety(ctx, ss) // escalation times are part of the saved plan
	}
	for _, act := range acts {
		reason := "missed_check_in"
		duress := ss.sess.Duress()
		switch {
		case act == meet.Emergency:
			reason = "panic"
		case duress:
			reason = "duress"
		}
		a := meet.Alert{ID: "al_" + randHex(8), Reason: reason, MemberAlias: ss.Alias, MeetWindow: window, Started: now}
		switch act {
		case meet.AlertTrusted:
			if to, alias := ss.Trusted, ss.Alias; to != "" {
				s.dispatch("trusted_contact", func(ctx context.Context) error {
					return s.Alerts.TrustedContact(ctx, to, alias, window, duress)
				})
			}
		case meet.AlertSafetyLine:
			s.dispatch("safety_line", func(ctx context.Context) error { return s.Alerts.SafetyLine(ctx, a) })
		case meet.Emergency:
			s.dispatch("emergency", func(ctx context.Context) error { return s.Alerts.Emergency(ctx, a) })
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
