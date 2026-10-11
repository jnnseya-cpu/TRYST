package meet

// Safety-line routing (FR-111, D-48).
//
// The 24/7 safety line is staffed from day one by a contracted alarm-receiving centre (ARC)
// accredited to BS EN 50518 and operating to BS 8484 (the UK lone-worker standard, whose
// alerts police treat with priority), with TRYST's on-call Trust & Safety rota as the backup.
// Every alert must be acknowledged by a person within the responder's deadline; otherwise it
// moves to the next responder. If the ARC stops sending heartbeats, new alerts go straight to
// the backup. If nobody acknowledges, the member's trusted contact is alerted and the member's
// phone shows the 999 / Silent Solution screen.

import (
	"errors"
	"time"
)

// Responder is one tier of the safety line.
type Responder struct {
	Name        string
	AckWithin   time.Duration
	heartbeat   time.Time
	HeartbeatOK time.Duration // 0 = no heartbeat required
}

// Alert is the minimum an ARC operator needs to help, and nothing more: no profile, no
// photos, no messages, no description of the service. The caller line ID and script are
// neutral ("personal safety service").
type Alert struct {
	ID            string
	Reason        string // missed_check_in | duress | panic
	MemberAlias   string // the pseudonym the member chose for the safety line, not their profile name
	CallbackPhone string // decrypted only for this alert
	Location      string // what the member shared for this meet only
	MeetWindow    string // "20:00–23:00"
	Started       time.Time
}

// Allowed lists the only fields that may ever be sent to a responder. Tests pin it.
var Allowed = []string{"ID", "Reason", "MemberAlias", "CallbackPhone", "Location", "MeetWindow", "Started"}

// Outcome is what happens next.
type Outcome string

const (
	Routed           Outcome = "routed"             // sitting with a responder, awaiting acknowledgement
	Acknowledged     Outcome = "acknowledged"       // a person is handling it
	FallbackToMember Outcome = "fallback_to_member" // nobody acknowledged: trusted contact + 999 screen
)

// Line routes alerts through responders in order.
type Line struct {
	tiers []*Responder
}

var ErrNoTiers = errors.New("the safety line needs at least one responder")

// NewLine returns a line with responders in priority order (ARC first, on-call second).
func NewLine(tiers ...*Responder) (*Line, error) {
	if len(tiers) == 0 {
		return nil, ErrNoTiers
	}
	return &Line{tiers: tiers}, nil
}

// Heartbeat records that a responder's link is alive.
func (l *Line) Heartbeat(name string, at time.Time) {
	for _, r := range l.tiers {
		if r.Name == name {
			r.heartbeat = at
		}
	}
}

func (r *Responder) healthy(now time.Time) bool {
	return r.HeartbeatOK == 0 || (!r.heartbeat.IsZero() && now.Sub(r.heartbeat) <= r.HeartbeatOK)
}

// Case tracks one alert through the tiers.
type Case struct {
	line     *Line
	Alert    Alert
	tier     int
	since    time.Time
	ackedBy  string
	fellBack bool
	Log      []string
}

// Raise opens a case at the first healthy responder.
func (l *Line) Raise(a Alert, now time.Time) *Case {
	c := &Case{line: l, Alert: a, tier: -1}
	c.advance(now)
	return c
}

func (c *Case) advance(now time.Time) {
	for c.tier+1 < len(c.line.tiers) {
		c.tier++
		r := c.line.tiers[c.tier]
		if r.healthy(now) {
			c.since = now
			c.Log = append(c.Log, "routed:"+r.Name)
			return
		}
		c.Log = append(c.Log, "skipped_unhealthy:"+r.Name)
	}
	c.tier = len(c.line.tiers)
	c.fellBack = true
	c.Log = append(c.Log, "fallback_to_member")
}

// Responder currently holding the case, or "".
func (c *Case) Responder() string {
	if c.fellBack || c.tier >= len(c.line.tiers) {
		return ""
	}
	return c.line.tiers[c.tier].Name
}

// Ack is a person at the current responder taking the case.
func (c *Case) Ack(responder string) bool {
	if c.ackedBy != "" || c.Responder() != responder {
		return false
	}
	c.ackedBy = responder
	c.Log = append(c.Log, "acknowledged:"+responder)
	return true
}

// Tick moves an unacknowledged case on when its deadline passes.
func (c *Case) Tick(now time.Time) Outcome {
	switch {
	case c.ackedBy != "":
		return Acknowledged
	case c.fellBack:
		return FallbackToMember
	}
	if now.Sub(c.since) > c.line.tiers[c.tier].AckWithin {
		c.Log = append(c.Log, "ack_timeout:"+c.line.tiers[c.tier].Name)
		c.advance(now)
		if c.fellBack {
			return FallbackToMember
		}
	}
	return Routed
}
