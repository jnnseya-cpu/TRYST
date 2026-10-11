package meet

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

func line(t *testing.T) (*Line, *Responder, *Responder) {
	t.Helper()
	arc := &Responder{Name: "arc", AckWithin: 60 * time.Second, HeartbeatOK: 5 * time.Minute}
	oncall := &Responder{Name: "oncall", AckWithin: 5 * time.Minute}
	l, err := NewLine(arc, oncall)
	if err != nil {
		t.Fatal(err)
	}
	return l, arc, oncall
}

var alert = Alert{ID: "a1", Reason: "duress", MemberAlias: "Juniper", CallbackPhone: "+447700900123", Location: "Bar on Upper St", MeetWindow: "20:00–23:00"}

func TestARCAcknowledges(t *testing.T) {
	l, _, _ := line(t)
	l.Heartbeat("arc", t0)
	c := l.Raise(alert, t0.Add(time.Minute))
	if c.Responder() != "arc" || !c.Ack("arc") || c.Tick(t0.Add(10*time.Minute)) != Acknowledged {
		t.Fatalf("log: %v", c.Log)
	}
	if c.Ack("oncall") {
		t.Fatal("a case is taken once")
	}
}

func TestUnansweredMovesToOnCallThenMember(t *testing.T) {
	l, _, _ := line(t)
	l.Heartbeat("arc", t0)
	c := l.Raise(alert, t0)
	if c.Tick(t0.Add(30*time.Second)) != Routed || c.Responder() != "arc" {
		t.Fatal("inside ARC deadline")
	}
	if c.Tick(t0.Add(61*time.Second)) != Routed || c.Responder() != "oncall" {
		t.Fatalf("after 60 s: %v", c.Log)
	}
	if c.Ack("arc") {
		t.Fatal("the ARC can no longer take it")
	}
	if c.Tick(t0.Add(7*time.Minute)) != FallbackToMember {
		t.Fatalf("nobody answered: %v", c.Log)
	}
}

func TestDeadARCIsSkipped(t *testing.T) {
	l, _, _ := line(t)
	l.Heartbeat("arc", t0)
	c := l.Raise(alert, t0.Add(6*time.Minute)) // heartbeat 6 minutes old
	if c.Responder() != "oncall" || !slices.Contains(c.Log, "skipped_unhealthy:arc") {
		t.Fatalf("log: %v", c.Log)
	}
}

// The alert type carries only what an operator needs; adding a field must be a deliberate,
// reviewed change to this test (no profile, photos, messages or service description).
func TestAlertCarriesMinimumData(t *testing.T) {
	var fields []string
	ty := reflect.TypeOf(Alert{})
	for i := 0; i < ty.NumField(); i++ {
		fields = append(fields, ty.Field(i).Name)
	}
	if !slices.Equal(fields, Allowed) {
		t.Fatalf("alert fields changed: %v", fields)
	}
	if _, err := NewLine(); err == nil {
		t.Fatal("empty line")
	}
}
