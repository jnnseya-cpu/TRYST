package vendors

import (
	"context"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/meet"
)

// Messenger sends one short neutral message to a phone number or email (outbound.Sender
// style routing by "@").
type Messenger interface {
	Message(ctx context.Context, to, text string) error
}

// ARCAlerting is profilesvc.Alerting for production (D-48, FR-111): missed check-ins and
// duress go to the contracted alarm-receiving centre; Panic goes to it at emergency
// priority (a BS 8484 ARC can pass it to police with its unique reference number); the
// trusted contact gets a neutral text. Only the pinned meet.Alert fields are ever sent.
type ARCAlerting struct {
	ARC     *Gateway
	Contact Messenger
}

func arcBody(a meet.Alert, priority string) map[string]any {
	return map[string]any{
		"id": a.ID, "reason": a.Reason, "priority": priority, "member_alias": a.MemberAlias,
		"callback_phone": a.CallbackPhone, "location": a.Location, "meet_window": a.MeetWindow,
		"started": a.Started.UTC().Format(time.RFC3339),
	}
}

// SafetyLine routes a missed check-in or duress alert. The ARC must answer 2xx with its own
// case reference; anything else is an error and the escalation ladder moves on.
func (a *ARCAlerting) SafetyLine(ctx context.Context, al meet.Alert) error {
	var out struct {
		Reference string `json:"reference"`
	}
	if err := a.ARC.Post(ctx, "/alerts", arcBody(al, "urgent"), &out); err != nil {
		return err
	}
	if out.Reference == "" {
		return ErrVendor
	}
	return nil
}

// Emergency routes Panic at the highest priority.
func (a *ARCAlerting) Emergency(ctx context.Context, al meet.Alert) error {
	var out struct {
		Reference string `json:"reference"`
	}
	if err := a.ARC.Post(ctx, "/alerts", arcBody(al, "emergency"), &out); err != nil {
		return err
	}
	if out.Reference == "" {
		return ErrVendor
	}
	return nil
}

// TrustedContactText is what the trusted contact receives. It names only the alias the
// member chose, never TRYST, and reads the same for duress so a coercer who sees the
// contact's phone learns nothing more; duress only makes it more urgent.
func TrustedContactText(alias, window string, duress bool) string {
	t := alias + " asked you to check on them if they didn't confirm they were safe by the end of " + window + ". They haven't. Please call them."
	if duress {
		t = alias + " asked you to check on them. Please call them now, and call 999 if you can't reach them or are worried."
	}
	return t
}

func (a *ARCAlerting) TrustedContact(ctx context.Context, to, alias, window string, duress bool) error {
	return a.Contact.Message(ctx, to, TrustedContactText(alias, window, duress))
}
