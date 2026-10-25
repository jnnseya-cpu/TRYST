// Package comms is TRYST's communication catalogue and delivery log: every message the
// platform sends (in-app notices, content-free push, email, the safety line), with its
// exact wording and channels, and a record of each delivery attempt for the staff
// console's Communications page.
//
// Discretion rules shape all of it (01 §9, 04 §5; D-29, D-55, D-67):
//   - In-app text is seen only inside the signed-in app and names nobody.
//   - Push carries no content: the device shows the member's own neutral wording.
//   - Email is plain text from a neutral sender, with no logo and no mention of TRYST,
//     because an inbox may be seen by someone else. TRYST sends no activity or marketing
//     email.
//   - No WhatsApp, ever; SMS only if an SMS provider is added (email-only at launch).
//   - The delivery log holds no address, no name and no content: recipients are short
//     pseudonyms that change on every restart.
package comms

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

// Channels.
const (
	InApp      = "inapp"
	Push       = "push"
	Email      = "email"
	SMS        = "sms"
	SafetyLine = "safety_line" // the alarm-receiving centre's API
)

// Event is one kind of message.
type Event struct {
	Key       string   `json:"key"`
	Category  string   `json:"category"`
	Label     string   `json:"label"`
	Severity  string   `json:"severity"` // info | success | warning | critical
	Audience  string   `json:"audience"` // member | trusted_contact | responders | staff
	Mandatory bool     `json:"mandatory"`
	Channels  []string `json:"channels"`
	Subject   string   `json:"subject,omitempty"` // email subject
	Text      string   `json:"text"`              // exact in-app or email text (sample values for variables)
	Kind      string   `json:"-"`                 // notify kind for in-app events
	Trigger   string   `json:"trigger"`           // what causes it
}

// Categories, in console order.
var Categories = []string{"Sign-in and account", "Introductions", "Messages and meets", "Safety", "Billing"}

// Catalogue is every message TRYST sends. In-app wording here is what members see
// (internal/notify reads it); email wording is checked against internal/outbound and
// internal/vendors by tests.
var Catalogue = []Event{
	{Key: "account.signup_code", Category: "Sign-in and account", Label: "Sign-up code", Severity: "info", Audience: "member", Mandatory: true,
		Channels: []string{Email}, Subject: "Your verification code",
		Text: "Your verification code is 482913. It expires in 10 minutes. If you didn't ask for it, ignore this message.", Trigger: "Someone starts joining with this email address"},
	{Key: "account.new_device", Kind: "new_device", Category: "Sign-in and account", Label: "New device signed in", Severity: "warning", Audience: "member", Mandatory: true,
		Channels: []string{InApp, Push}, Text: "A new device signed in to your account. If this wasn't you, use Burn.", Trigger: "A passkey is added on another device"},
	{Key: "account.recovery_started", Kind: "recovery_started", Category: "Sign-in and account", Label: "Recovery started", Severity: "critical", Audience: "member", Mandatory: true,
		Channels: []string{InApp, Push}, Text: "Someone started recovering your account. If this wasn't you, cancel it in your account.", Trigger: "Account recovery passes its checks"},
	{Key: "account.recovery_hold", Kind: "recovery_hold", Category: "Sign-in and account", Label: "Recovery in 24 hours", Severity: "critical", Audience: "member", Mandatory: true,
		Channels: []string{InApp, Push}, Text: "Your account will be recovered in 24 hours. If this wasn't you, cancel it in your account now.", Trigger: "The 24-hour recovery hold begins"},

	{Key: "intro.new", Kind: "intro", Category: "Introductions", Label: "New introduction", Severity: "info", Audience: "member",
		Channels: []string{InApp, Push}, Text: "You have a new introduction.", Trigger: "A handshake opens for the member"},
	{Key: "intent.received", Kind: "intent", Category: "Introductions", Label: "Private intent received", Severity: "info", Audience: "member",
		Channels: []string{InApp, Push}, Text: "Someone would like to meet you.", Trigger: "An intent is delivered (inside the inbound cap)"},
	{Key: "match.new", Kind: "match", Category: "Introductions", Label: "New match", Severity: "success", Audience: "member",
		Channels: []string{InApp, Push}, Text: "You have a new match.", Trigger: "Both accept a handshake, or Tonight interest is mutual"},
	{Key: "couple.update", Kind: "couple", Category: "Introductions", Label: "Couple link update", Severity: "info", Audience: "member",
		Channels: []string{InApp, Push}, Text: "There's an update about your couple link.", Trigger: "An invitation, confirmation, veto-mode change or end of the link"},

	{Key: "message.new", Kind: "message", Category: "Messages and meets", Label: "New message", Severity: "info", Audience: "member",
		Channels: []string{InApp, Push}, Text: "You have a new message.", Trigger: "An encrypted message arrives (push coalesced to one per 2 minutes)"},
	{Key: "meet.proposed", Kind: "meet_proposed", Category: "Messages and meets", Label: "Meet proposed", Severity: "info", Audience: "member",
		Channels: []string{InApp, Push}, Text: "A meet was proposed.", Trigger: "The other member proposes a meet"},
	{Key: "meet.updated", Kind: "meet_update", Category: "Messages and meets", Label: "Meet confirmed or changed", Severity: "info", Audience: "member",
		Channels: []string{InApp, Push}, Text: "A meet was confirmed or changed.", Trigger: "A meet is confirmed or cancelled"},

	{Key: "safety.checkin_missed", Category: "Safety", Label: "Missed check-in (trusted contact)", Severity: "critical", Audience: "trusted_contact", Mandatory: true,
		Channels: []string{Email}, Subject: "Please check on a friend",
		Text: "Sam asked you to check on them if they didn't confirm they were safe by the end of 20:00–23:00 UK time. They haven't. Please call them.", Trigger: "A check-in is missed and the grace period ends"},
	{Key: "safety.duress", Category: "Safety", Label: "Duress PIN (trusted contact)", Severity: "critical", Audience: "trusted_contact", Mandatory: true,
		Channels: []string{Email}, Subject: "Please check on a friend",
		Text: "Sam asked you to check on them. Please call them now, and call 999 if you can't reach them or are worried.", Trigger: "The duress PIN is entered"},
	{Key: "safety.line_alert", Category: "Safety", Label: "Safety line alert", Severity: "critical", Audience: "responders", Mandatory: true,
		Channels: []string{SafetyLine}, Text: "Alias, meet window and reason (missed check-in or duress) to the alarm-receiving centre; nothing else about the member.", Trigger: "Escalation after the trusted contact, or duress, when the member opted in"},
	{Key: "safety.panic", Category: "Safety", Label: "Panic", Severity: "critical", Audience: "responders", Mandatory: true,
		Channels: []string{SafetyLine}, Text: "Emergency to the alarm-receiving centre with the location the member chose to share; the member is shown 999 and the Silent Solution.", Trigger: "The member presses Panic"},
	{Key: "safety.update", Kind: "safety", Category: "Safety", Label: "Safety team update", Severity: "warning", Audience: "member", Mandatory: true,
		Channels: []string{InApp, Push}, Text: "There's an update from our safety team. See your account.", Trigger: "A moderation decision about the member"},
	{Key: "safety.appeal_decided", Kind: "appeal", Category: "Safety", Label: "Appeal decided", Severity: "info", Audience: "member", Mandatory: true,
		Channels: []string{InApp, Push}, Text: "Your appeal has been decided. See your account.", Trigger: "A different reviewer decides the appeal"},

	{Key: "billing.payment_received", Kind: "payment", Category: "Billing", Label: "Payment received", Severity: "success", Audience: "member",
		Channels: []string{InApp, Push}, Text: "Payment received.", Trigger: "The processor confirms a payment"},
	{Key: "billing.renewal_reminder", Kind: "renewal", Category: "Billing", Label: "Renewal reminder", Severity: "warning", Audience: "member", Mandatory: true,
		Channels: []string{InApp, Push}, Text: "Your plan renews soon. You can cancel any time in Plans and Keys.", Trigger: "Before each renewal (the processor also emails the billing address, at least 7 days ahead)"},
	{Key: "billing.plan_ended", Kind: "plan_ended", Category: "Billing", Label: "Plan ended", Severity: "info", Audience: "member",
		Channels: []string{InApp, Push}, Text: "Your plan has ended.", Trigger: "A plan is cancelled or lapses"},
}

// NotSent is what TRYST deliberately never sends, and why, so nobody adds it by accident.
var NotSent = []struct {
	What string `json:"what"`
	Why  string `json:"why"`
}{
	{"WhatsApp messages", "A shared phone would show them, and a third party would learn who uses TRYST (D-29)."},
	{"Activity or marketing email", "An inbox may be seen by someone else. Members hear about activity only inside the app."},
	{"Branded email (logo, brand colours)", "Every email is plain text from a neutral sender that never says TRYST."},
	{"Push with content", "Pushes carry no names or text; the device shows the member's own neutral wording."},
	{"SMS", "Not at launch: sign-up is email-only (D-67). Adding an SMS provider turns on codes and trusted-contact alerts by text."},
}

// ByKind finds the in-app event for a notify kind.
func ByKind(kind string) (Event, bool) {
	for _, e := range Catalogue {
		if e.Kind == kind && kind != "" {
			return e, true
		}
	}
	return Event{}, false
}

// Lookup finds an event by key.
func Lookup(key string) (Event, bool) {
	for _, e := range Catalogue {
		if e.Key == key {
			return e, true
		}
	}
	return Event{}, false
}

// Delivery statuses.
const (
	Sent       = "sent"       // accepted by the channel (inbox stored, provider accepted)
	Failed     = "failed"     // the provider refused or was unreachable
	Sandbox    = "sandbox"    // development: recorded, nothing left the server
	Suppressed = "suppressed" // not sent by rule (push off, coalesced)
)

// Delivery is one attempt. It never holds an address, a name or content.
type Delivery struct {
	At        time.Time `json:"at"`
	Event     string    `json:"event"`
	Channel   string    `json:"channel"`
	Status    string    `json:"status"`
	Provider  string    `json:"provider"`
	Recipient string    `json:"recipient"` // short pseudonym, new on every restart
	Test      bool      `json:"test,omitempty"`
}

// Log keeps the most recent deliveries in memory, and totals since start.
type Log struct {
	mu     sync.Mutex
	recent []Delivery
	totals map[string]map[string]int // event → status → count
	salt   []byte
	now    func() time.Time
}

const keepRecent = 300

// Default is the process-wide log.
var Default = NewLog()

func NewLog() *Log {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	return &Log{totals: map[string]map[string]int{}, salt: salt, now: time.Now}
}

// Pseudonym turns an account id or address into a short tag that can't be reversed and
// changes on every restart.
func (l *Log) Pseudonym(kind, id string) string {
	if id == "" {
		return kind
	}
	h := sha256.Sum256(append(append([]byte{}, l.salt...), id...))
	return kind + "·" + hex.EncodeToString(h[:3])
}

// Record adds a delivery. recipientKind is "member", "contact" or "staff"; recipientID is
// hashed into a pseudonym and not kept.
func (l *Log) Record(event, channel, status, provider, recipientKind, recipientID string, test bool) {
	if l == nil {
		return
	}
	d := Delivery{At: l.now(), Event: event, Channel: channel, Status: status, Provider: provider,
		Recipient: l.Pseudonym(recipientKind, recipientID), Test: test}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.recent = append([]Delivery{d}, l.recent...)
	if len(l.recent) > keepRecent {
		l.recent = l.recent[:keepRecent]
	}
	if l.totals[event] == nil {
		l.totals[event] = map[string]int{}
	}
	l.totals[event][status]++
}

// Snapshot is what the console shows.
type Snapshot struct {
	Recent []Delivery                `json:"recent"`
	Totals map[string]map[string]int `json:"totals"`
}

func (l *Log) Snapshot(limit int) Snapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	if limit <= 0 || limit > len(l.recent) {
		limit = len(l.recent)
	}
	out := Snapshot{Recent: append([]Delivery{}, l.recent[:limit]...), Totals: map[string]map[string]int{}}
	for e, m := range l.totals {
		out.Totals[e] = map[string]int{}
		for k, v := range m {
			out.Totals[e][k] = v
		}
	}
	return out
}

// Coverage counts catalogue events per channel.
func Coverage() map[string]int {
	out := map[string]int{}
	for _, e := range Catalogue {
		for _, c := range e.Channels {
			out[c]++
		}
	}
	return out
}

// Keys lists event keys in catalogue order (tests).
func Keys() []string {
	out := make([]string, len(Catalogue))
	for i, e := range Catalogue {
		out[i] = e.Key
	}
	sort.Strings(out)
	return out
}
