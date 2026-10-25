package safetysvc

import (
	"context"
	"strings"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/comms"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

type fakeComms struct{ tests []string }

func (f *fakeComms) Snapshot(context.Context) (comms.Snapshot, string, error) {
	return comms.Snapshot{Recent: []comms.Delivery{{Event: "message.new", Channel: comms.InApp, Status: comms.Sent, Recipient: "member·a1b2c3"}}}, "smtp smtp.hostinger.com", nil
}
func (f *fakeComms) Test(_ context.Context, event, to string) ([]map[string]string, error) {
	f.tests = append(f.tests, event+"→"+to)
	return []map[string]string{{"channel": "email", "status": "sent"}}, nil
}

func TestCommsPageStaffReadLeadTests(t *testing.T) {
	g := newRig(t, store.NewMemory())
	fc := &fakeComms{}
	g.svc.Comms = fc

	w, out := g.do("GET", "/ops/v1/comms", "", tokMod1, nil)
	if w.Code != 200 || len(out["catalogue"].([]any)) != len(comms.Catalogue) || out["email_provider"] != "smtp smtp.hostinger.com" {
		t.Fatalf("read: %d %v", w.Code, out["email_provider"])
	}
	if !strings.Contains(w.Body.String(), "WhatsApp") {
		t.Fatal("the never-sent list is missing")
	}
	if w, _ := g.do("GET", "/ops/v1/comms", "", "", nil); w.Code != 401 {
		t.Fatalf("signed out: %d", w.Code)
	}

	if w, _ := g.do("POST", "/ops/v1/comms/test", "", tokMod1, map[string]any{"event": "account.signup_code"}); w.Code != 403 {
		t.Fatalf("moderator test: %d", w.Code)
	}
	if w, _ := g.do("POST", "/ops/v1/comms/test", "", tokLead, map[string]any{"event": "nope"}); w.Code != 404 {
		t.Fatalf("unknown event: %d", w.Code)
	}
	if w, _ := g.do("POST", "/ops/v1/comms/test", "", tokLead, map[string]any{"event": "account.signup_code"}); w.Code != 200 {
		t.Fatalf("lead test: %d", w.Code)
	}
	// A test only ever goes to the lead themselves.
	if len(fc.tests) != 1 || fc.tests[0] != "account.signup_code→lead1" {
		t.Fatalf("tests: %v", fc.tests)
	}
	last := g.svc.chain[len(g.svc.chain)-1]
	if last.Action != "comms_test" || last.Detail != "account.signup_code" {
		t.Fatalf("not audited: %+v", last)
	}
}
