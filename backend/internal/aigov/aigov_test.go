package aigov

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

func loaded(t *testing.T) *Gate {
	t.Helper()
	g := New(func() time.Time { return t0 }, "oncall.lead", "tns.lead")
	for _, m := range Workforce() {
		if err := g.Register(m); err != nil {
			t.Fatalf("register %s: %v", m.ID, err)
		}
	}
	return g
}

func TestWorkforceLoads(t *testing.T) {
	g := loaded(t)
	if n := len(g.Agents()); n != len(Workforce()) {
		t.Fatalf("registered %d agents", n)
	}
}

// No autonomy level unlocks the always-human classes (P4, Art 22).
func TestAlwaysHumanClassesNeedAPerson(t *testing.T) {
	g := loaded(t)
	cases := []struct{ agent, action string }{
		{"guardian", "remove_member"}, {"guardian", "freeze_account"}, {"risk", "ban_anchor_match"},
		{"fraud", "refund"}, {"payment", "change_price"}, {"support", "issue_goodwill_keys"},
		{"compliance", "erase_account"}, {"herald", "publish_page"},
		{"auto-repair", "deploy_patch"}, {"release-manager", "promote_release"}, {"infra-optimiser", "apply_rightsizing"},
	}
	for _, c := range cases {
		if d := g.Decide(Request{AgentID: c.agent, Action: c.action}); d.Outcome != NeedsHuman {
			t.Errorf("%s/%s without approver = %s (%s), want needs_human", c.agent, c.action, d.Outcome, d.Reason)
		}
		if d := g.Decide(Request{AgentID: c.agent, Action: c.action, Approver: "tns.lead"}); d.Outcome != Allow {
			t.Errorf("%s/%s with approver = %s", c.agent, c.action, d.Outcome)
		}
	}
}

func TestUnknownApproverIsDenied(t *testing.T) {
	g := loaded(t)
	d := g.Decide(Request{AgentID: "guardian", Action: "remove_member", Approver: "guardian"})
	if d.Outcome != Deny {
		t.Fatalf("an agent cannot approve itself: %s", d.Outcome)
	}
}

func TestLevelCapsAutonomy(t *testing.T) {
	g := loaded(t)
	if d := g.Decide(Request{AgentID: "envoy", Action: "propose_outreach"}); d.Outcome != Allow {
		t.Fatalf("recommend at L1: %s", d.Outcome)
	}
	if d := g.Decide(Request{AgentID: "auto-repair", Action: "rollback_release"}); d.Outcome != Allow {
		t.Fatalf("runbook at L3: %s", d.Outcome)
	}
	if d := g.Decide(Request{AgentID: "curtain", Action: "nonexistent"}); d.Outcome != Deny {
		t.Fatalf("unlisted action: %s", d.Outcome)
	}
	if d := g.Decide(Request{AgentID: "ghost-agent", Action: "read"}); d.Outcome != Deny {
		t.Fatalf("unknown agent: %s", d.Outcome)
	}
}

func TestBudgetThenHuman(t *testing.T) {
	g := loaded(t)
	for i := 0; i < 50; i++ {
		if d := g.Decide(Request{AgentID: "auto-repair", Action: "restart_service"}); d.Outcome != Allow {
			t.Fatalf("run %d: %s", i, d.Outcome)
		}
	}
	if d := g.Decide(Request{AgentID: "auto-repair", Action: "restart_service"}); d.Outcome != NeedsHuman {
		t.Fatalf("over budget: %s", d.Outcome)
	}
}

func TestKillSwitch(t *testing.T) {
	g := loaded(t)
	g.Kill("broker", "oncall.lead")
	if d := g.Decide(Request{AgentID: "broker", Action: "compute_slate"}); d.Outcome != Deny {
		t.Fatalf("killed agent ran: %s", d.Outcome)
	}
	g.Resume("broker", "oncall.lead")
	if d := g.Decide(Request{AgentID: "broker", Action: "compute_slate"}); d.Outcome != Allow {
		t.Fatalf("resumed: %s", d.Outcome)
	}
	g.KillAll("oncall.lead")
	if d := g.Decide(Request{AgentID: "guardian", Action: "remove_member", Approver: "tns.lead"}); d.Outcome != Deny {
		t.Fatalf("kill-all must stop even approved actions: %s", d.Outcome)
	}
}

func TestManifestRules(t *testing.T) {
	g := New(nil)
	bad := []Manifest{
		{ID: "x", Owner: "o", Actions: map[string]Class{"talk": ActAsMember}},
		{ID: "y", Owner: "o", Actions: map[string]Class{"r": Read}, DataScopes: []string{"profile_db"}},
		{ID: "z", Owner: "o", Actions: map[string]Class{"r": Read}, DataScopes: []string{"profile_db"}, MemberPlane: true},
		{ID: "", Owner: "o", Actions: map[string]Class{"r": Read}},
	}
	for _, m := range bad {
		if err := g.Register(m); !errors.Is(err, ErrInvalidAgent) {
			t.Errorf("%q: got %v, want ErrInvalidAgent", m.ID, err)
		}
	}
}

// Agents outside the member platform never hold member data (FR-075, FR-081).
func TestOpsAgentsHoldNoMemberScopes(t *testing.T) {
	for _, m := range Workforce() {
		for _, s := range m.DataScopes {
			if memberScopes[s] && (!m.MemberPlane || !m.SelfHosted) {
				t.Errorf("%s holds %s", m.ID, s)
			}
		}
		if m.ID == "herald" && m.MemberPlane {
			t.Error("herald must be outside the member platform")
		}
	}
}

func TestAuditChain(t *testing.T) {
	g := loaded(t)
	g.Decide(Request{AgentID: "guardian", Action: "remove_member", Subject: "sub_9f2", Approver: "tns.lead"})
	log := g.Audit()
	if err := VerifyChain(log); err != nil {
		t.Fatal(err)
	}
	last := log[len(log)-1]
	if last.Approver != "tns.lead" || !strings.Contains(last.Detail, "sub_9f2") {
		t.Fatalf("approval not recorded: %+v", last)
	}
	log[3].Detail = "edited"
	if err := VerifyChain(log); !errors.Is(err, ErrAuditTampered) {
		t.Fatalf("tamper not detected: %v", err)
	}
	if err := VerifyChain(append(g.Audit()[:2], g.Audit()[3:]...)); !errors.Is(err, ErrAuditTampered) {
		t.Fatalf("deletion not detected: %v", err)
	}
}
