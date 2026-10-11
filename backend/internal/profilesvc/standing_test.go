package profilesvc

import (
	"context"
	"testing"
)

func TestSuspensionLocksOutAndClosesConversations(t *testing.T) {
	w, tid := mutualPair(t)
	if who, ok := w.s.Counterpart("acc-ana", tid); !ok || who != "acc-ben" {
		t.Fatalf("counterpart = %q %v", who, ok)
	}
	if _, ok := w.s.Counterpart("acc-eve", tid); ok {
		t.Fatal("counterpart for an outsider")
	}
	if name, area := w.s.Summary("acc-ben"); name != "Ben" || area == "" {
		t.Fatalf("summary = %q %q", name, area)
	}
	if b, ct, err := w.s.MainPhoto(context.Background(), "acc-ben"); err != nil || len(b) == 0 || ct != "image/jpeg" {
		t.Fatalf("main photo: %v %q", err, ct)
	}

	w.s.SetStanding("acc-ben", "suspend")
	if code, out := w.do("ben", "GET", "/v1/threads", ""); code != 403 || out["code"] != "account_suspended" {
		t.Fatalf("suspended member: %d %v", code, out)
	}
	if code, _ := w.do("ben", "GET", "/v1/profile", ""); code != 403 {
		t.Fatalf("suspended profile: %d", code)
	}
	_, out := w.do("ana", "GET", "/v1/threads/"+tid, "")
	if out["thread"].(map[string]any)["state"] != "closed" {
		t.Fatalf("thread with a suspended member: %v", out)
	}
	if code, _ := send(w, "ana", tid, "dev-ana-0001", "c1", "dev-ben-0001"); code != 409 {
		t.Fatalf("send to suspended member: %d", code)
	}
	for _, p := range people(func() map[string]any { _, o := w.do("eve", "GET", "/v1/tonight", ""); return o }(), "people") {
		if p["display_name"] == "Ben" {
			t.Fatal("suspended member shown in Tonight")
		}
	}

	// An overturned decision restores everything.
	w.s.SetStanding("acc-ben", "good")
	if code, _ := w.do("ben", "GET", "/v1/threads", ""); code != 200 {
		t.Fatalf("restored member: %d", code)
	}

	// Shadow limit: keeps conversations, hidden from Tonight.
	w.s.SetStanding("acc-ben", "shadow_limit")
	if code, _ := w.do("ben", "GET", "/v1/threads", ""); code != 200 {
		t.Fatalf("shadow-limited member: %d", code)
	}
	_, o := w.do("eve", "GET", "/v1/tonight", "")
	for _, p := range people(o, "people") {
		if p["display_name"] == "Ben" {
			t.Fatal("shadow-limited member shown in Tonight")
		}
	}

	// Standing survives a restart.
	w.s.SetStanding("acc-ben", "permanent")
	s2, err := NewWithStore(w.s.auth, w.s.mod, "dev", w.s.st, w.s.vault)
	if err != nil {
		t.Fatal(err)
	}
	if s2.standing["acc-ben"] != "permanent" {
		t.Fatalf("standing after restart = %q", s2.standing["acc-ben"])
	}
}

func TestSignalSubjectIsTheSender(t *testing.T) {
	w, tid := mutualPair(t)
	var got [][2]string
	w.s.OnSignal = func(_, reporter, subject, _ string) { got = append(got, [2]string{reporter, subject}) }
	w.do("ana", "POST", "/v1/threads/"+tid+"/signal", `{"kind":"extortion","model":"baseline-v1","about":"received"}`)
	w.do("ben", "POST", "/v1/threads/"+tid+"/signal", `{"kind":"child_safety","model":"baseline-v1","about":"blocked_send"}`)
	if code, _ := w.do("ben", "POST", "/v1/threads/"+tid+"/signal", `{"kind":"fraud","model":"baseline-v1","about":"other"}`); code != 422 {
		t.Fatalf("unknown about: %d", code)
	}
	want := [][2]string{{"acc-ana", "acc-ben"}, {"acc-ben", "acc-ben"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("signals = %v, want %v", got, want)
	}
}
