// Package meet implements the safety layer around meeting someone in person
// (docs/spec/01_Product.md §8.3; 04 §7; FR-020–FR-022, FR-099, FR-100).
//
//   - Door check: when both members arrive, each phone shows the other's verified main photo
//     and the same three words, derived from the handshake secret only the two phones hold.
//     If the person in front of you does not match, you leave: no conversation needed.
//   - Check-ins: during a meet the member confirms they are fine at the interval they chose,
//     with their own check-in PIN. A separate duress PIN looks exactly like a normal check-in
//     on screen but silently starts the escalation. A missed check-in escalates too.
//   - Escalation goes first to the member's trusted contact (stored on their phone only),
//     then, if the member opted in, to TRYST's 24/7 safety team, who receive the location
//     shared for this meet only. Panic skips straight to emergency escalation (P7).
//
// TRYST never learns where a meet is unless the member opts in to the safety line or uses
// Panic; the venue lives in the encrypted thread and in Share My Plan.
package meet

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"time"
)

// words is a short list of distinct, easy-to-say words (one per 6 bits used).
var words = [64]string{
	"amber", "anchor", "apple", "arrow", "autumn", "badge", "bamboo", "banner",
	"basil", "beacon", "birch", "blossom", "bridge", "bronze", "cabin", "canyon",
	"cedar", "cello", "cherry", "cobalt", "comet", "coral", "cotton", "crystal",
	"dahlia", "delta", "ember", "falcon", "feather", "fern", "fjord", "garnet",
	"ginger", "glacier", "harbor", "hazel", "heron", "indigo", "island", "ivory",
	"jasmine", "juniper", "kettle", "lagoon", "lantern", "lemon", "linen", "lotus",
	"maple", "marble", "meadow", "mint", "nectar", "olive", "orchid", "pebble",
	"pepper", "quartz", "raven", "saffron", "silver", "tulip", "velvet", "willow",
}

// DoorWords derives the three words both phones show. Same inputs on both phones → same
// words; anyone without the handshake secret cannot produce them.
func DoorWords(handshakeSecret []byte, meetID string) [3]string {
	m := hmac.New(sha256.New, handshakeSecret)
	m.Write([]byte("tryst/meet/door/v1\x00" + meetID))
	sum := m.Sum(nil)
	n := binary.BigEndian.Uint32(sum[:4])
	return [3]string{words[n>>26&63], words[n>>20&63], words[n>>14&63]}
}

// Plan is what the member set before the meet. Contacts stay on the phone.
type Plan struct {
	Start        time.Time
	CheckInEvery time.Duration // member's choice, 15 min to 3 h
	Grace        time.Duration // time after a missed check-in before anyone is told
	End          time.Time
	SafetyLine   bool // opted in to TRYST's 24/7 safety team as the second step
	HasTrusted   bool // a trusted contact is set on the phone
	pinHash      []byte
	duressHash   []byte
}

var (
	ErrPinsMustDiffer = errors.New("the duress PIN must be different from the check-in PIN")
	ErrNoEscalation   = errors.New("set a trusted contact or turn on the safety line before the meet")
	ErrInterval       = errors.New("check-in interval must be between 15 minutes and 3 hours")
)

func hashPIN(salt []byte, pin string) []byte {
	m := hmac.New(sha256.New, salt)
	m.Write([]byte(pin))
	return m.Sum(nil)
}

// NewPlan validates the plan. salt is device-local.
func NewPlan(start, end time.Time, every time.Duration, pin, duressPIN string, salt []byte, trusted, safetyLine bool) (*Plan, error) {
	if every < 15*time.Minute || every > 3*time.Hour {
		return nil, ErrInterval
	}
	if pin == duressPIN || pin == "" || duressPIN == "" {
		return nil, ErrPinsMustDiffer
	}
	if !trusted && !safetyLine {
		return nil, ErrNoEscalation
	}
	return &Plan{Start: start, End: end, CheckInEvery: every, Grace: 10 * time.Minute,
		SafetyLine: safetyLine, HasTrusted: trusted, pinHash: hashPIN(salt, pin), duressHash: hashPIN(salt, duressPIN)}, nil
}

// Action is something the safety layer must do now.
type Action string

const (
	None            Action = ""
	AlertTrusted    Action = "alert_trusted_contact" // neutral, unbranded message with venue and time window
	AlertSafetyLine Action = "alert_safety_team"     // TRYST 24/7 team: call the member, then emergency services
	Emergency       Action = "emergency_escalation"  // Panic: 999 guidance, live location, evidence preserved
)

// Session tracks one meet.
type Session struct {
	plan      *Plan
	salt      []byte
	lastOK    time.Time
	duressAt  time.Time
	alertedAt time.Time
	safetyAt  time.Time
	emergency bool
	ended     bool
}

func Start(p *Plan, salt []byte) *Session { return &Session{plan: p, salt: salt, lastOK: p.Start} }

// State is everything a Session needs to carry on after a restart. It holds PIN hashes,
// never PINs.
type State struct {
	Start, End       time.Time
	Every, Grace     time.Duration
	SafetyLine       bool
	HasTrusted       bool
	PINHash          []byte
	DuressHash       []byte
	Salt             []byte
	LastOK, DuressAt time.Time
	AlertedAt        time.Time
	SafetyAt         time.Time
	Emergency, Ended bool
}

// State snapshots the session for saving.
func (s *Session) State() State {
	p := s.plan
	return State{Start: p.Start, End: p.End, Every: p.CheckInEvery, Grace: p.Grace, SafetyLine: p.SafetyLine,
		HasTrusted: p.HasTrusted, PINHash: p.pinHash, DuressHash: p.duressHash, Salt: s.salt, LastOK: s.lastOK,
		DuressAt: s.duressAt, AlertedAt: s.alertedAt, SafetyAt: s.safetyAt, Emergency: s.emergency, Ended: s.ended}
}

// Resume rebuilds a session saved with State; its timers carry on from where they were.
func Resume(st State) *Session {
	p := &Plan{Start: st.Start, End: st.End, CheckInEvery: st.Every, Grace: st.Grace, SafetyLine: st.SafetyLine,
		HasTrusted: st.HasTrusted, pinHash: st.PINHash, duressHash: st.DuressHash}
	return &Session{plan: p, salt: st.Salt, lastOK: st.LastOK, duressAt: st.DuressAt, alertedAt: st.AlertedAt,
		safetyAt: st.SafetyAt, emergency: st.Emergency, ended: st.Ended}
}

// CheckIn records a check-in. The screen always shows "Thanks, checked in", whichever PIN
// was used: the person with them cannot tell a duress check-in from a normal one.
func (s *Session) CheckIn(pin string, now time.Time) (screen string) {
	h := hashPIN(s.salt, pin)
	switch {
	case subtle.ConstantTimeCompare(h, s.plan.duressHash) == 1:
		if s.duressAt.IsZero() {
			s.duressAt = now
		}
	case subtle.ConstantTimeCompare(h, s.plan.pinHash) == 1:
		s.lastOK = now
	default:
		return "That PIN didn't match. Try again."
	}
	return "Thanks, checked in."
}

// Panic is the visible emergency control.
func (s *Session) Panic() { s.emergency = true }

// End closes the meet (the member confirms they are home or safe, with their check-in PIN).
func (s *Session) End(pin string) bool {
	if subtle.ConstantTimeCompare(hashPIN(s.salt, pin), s.plan.pinHash) == 1 {
		s.ended = true
	}
	return s.ended
}

// Tick returns the actions due at now. Call it every minute and on every event.
// Duress reports whether the duress PIN was used, so responders are told how urgent it is.
func (s *Session) Duress() bool { return !s.duressAt.IsZero() }

func (s *Session) Tick(now time.Time) []Action {
	if s.ended && !s.emergency {
		return nil
	}
	var out []Action
	if s.emergency {
		s.emergency = false
		return []Action{Emergency, AlertTrusted}
	}
	trigger := time.Time{}
	if !s.duressAt.IsZero() {
		trigger = s.duressAt // duress: no grace period
	} else if overdue := s.lastOK.Add(s.plan.CheckInEvery + s.plan.Grace); now.After(overdue) {
		trigger = overdue
	}
	if trigger.IsZero() {
		return nil
	}
	if s.alertedAt.IsZero() {
		s.alertedAt = now
		if s.plan.HasTrusted {
			out = append(out, AlertTrusted)
		}
		if !s.plan.HasTrusted && s.plan.SafetyLine {
			s.safetyAt = now
			out = append(out, AlertSafetyLine)
		}
	}
	if s.plan.SafetyLine && s.safetyAt.IsZero() && (now.Sub(s.alertedAt) >= 15*time.Minute || !s.duressAt.IsZero()) {
		s.safetyAt = now
		out = append(out, AlertSafetyLine)
	}
	return out
}
