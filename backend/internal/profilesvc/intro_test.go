package profilesvc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/keys"
)

// fakeKeys stands in for commerce.
type fakeKeys struct {
	mu      sync.Mutex
	balance map[string]int
	held    map[string]int
	plans   map[string]string
}

func (f *fakeKeys) spend(_ context.Context, a, intent string, n int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.balance[a] < n {
		return keys.ErrInsufficient
	}
	f.balance[a] -= n
	f.held[intent] = n
	return nil
}

func (f *fakeKeys) refund(_ context.Context, a, intent string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.balance[a] += f.held[intent]
	f.held[intent] = 0
}

func introWorld(t *testing.T, members ...string) (*world, *fakeKeys) {
	w := newWorld(t, "dev", members...)
	f := &fakeKeys{balance: map[string]int{}, held: map[string]int{}, plans: map[string]string{}}
	w.s.Plan = func(a string) string { f.mu.Lock(); defer f.mu.Unlock(); return f.plans[a] }
	w.s.SpendKeys, w.s.RefundKeys = f.spend, f.refund
	return w, f
}

func (w *world) intake(who, identity, seeking, shapes, limits string) {
	body := fmt.Sprintf(`{"identity":"%s","seeking":[%s],"intent_shape":[%s],"hard_limits":[%s],"interests":["dinner","art"]}`, identity, seeking, shapes, limits)
	if c, out := w.do(who, "POST", "/v1/intake/vector/confirm", body); c != 200 {
		w.t.Fatalf("intake %s: %d %v", who, c, out)
	}
}

func (w *world) slate(who string) map[string]string {
	c, out := w.do(who, "GET", "/v1/slate", "")
	if c != 200 {
		w.t.Fatalf("slate %s: %d %v", who, c, out)
	}
	names := map[string]string{}
	for _, cd := range out["cards"].([]any) {
		m := cd.(map[string]any)
		names[m["display_name"].(string)] = m["handle"].(string)
		if n := len(m["explanation_codes"].([]any)); n < 2 || n > 4 || m["paid_label"] != false {
			w.t.Fatalf("card: %v", m)
		}
	}
	return names
}

func TestSlateRunsStage0Gates(t *testing.T) {
	w, _ := introWorld(t, "ana", "ben", "cy", "dee", "eli")
	for who, d := range map[string]string{"ana": "Islington", "ben": "Camden", "cy": "Hackney", "dee": "City Centre", "eli": "Shoreditch"} {
		w.ready(who, d)
	}
	if c, out := w.do("ana", "GET", "/v1/slate", ""); c != 409 || out["code"] != "intake_required" {
		t.Fatalf("before intake: %d %v", c, out)
	}
	if c, _ := w.do("ana", "POST", "/v1/intake/vector/confirm", `{"identity":"woman","seeking":["men"],"intent_shape":["one_off"],"hard_limits":["one_off"],"interests":[]}`); c != 422 {
		t.Fatal("a hard limit equal to a wanted shape was accepted")
	}
	w.intake("ana", "woman", `"men"`, `"one_off","ongoing"`, `"third"`)
	w.intake("ben", "man", `"women"`, `"ongoing"`, ``)
	w.intake("cy", "man", `"men"`, `"ongoing"`, ``)            // doesn't seek women
	w.intake("dee", "man", `"women"`, `"ongoing"`, ``)         // Manchester
	w.intake("eli", "man", `"women"`, `"ongoing","third"`, ``) // crosses Ana's hard limit
	s := w.slate("ana")
	if _, ok := s["Ben"]; !ok || len(s) != 1 {
		t.Fatalf("Ana's slate: %v", s)
	}
	// Both ways: Ben sees Ana.
	if _, ok := w.slate("ben")["Ana"]; !ok {
		t.Fatal("Ben should see Ana")
	}
	// Passing is private and sticks.
	w.do("ana", "POST", "/v1/pass", `{"handle":"`+s["Ben"]+`"}`)
	if len(w.slate("ana")) != 0 {
		t.Fatal("pass ignored")
	}
	// Each confirmed version is kept.
	w.intake("ana", "woman", `"men"`, `"ongoing"`, ``)
	if _, out := w.do("ana", "GET", "/v1/intake/vector", ""); out["history"].(float64) != 2 {
		t.Fatalf("history: %v", out["history"])
	}
}

func TestIntentsFreeQuotaKeysAndRefundOnReply(t *testing.T) {
	names := []string{"ana", "b1", "b2", "b3", "b4", "b5"}
	w, f := introWorld(t, names...)
	w.ready("ana", "Islington")
	w.intake("ana", "woman", `"men"`, `"ongoing"`, ``)
	for i, b := range names[1:] {
		w.ready(b, []string{"Camden", "Hackney", "Shoreditch", "Islington", "Camden"}[i])
		w.intake(b, "man", `"women"`, `"ongoing"`, ``)
	}
	s := w.slate("ana")
	send := func(n string) (int, map[string]any) {
		return w.do("ana", "POST", "/v1/intent", `{"target":"`+s[n]+`"}`)
	}
	for _, n := range []string{"B1", "B2", "B3"} {
		if c, out := send(n); c != 202 || out["state"] != "sent" {
			t.Fatalf("free intent to %s: %d %v", n, c, out)
		}
	}
	if c, out := send("B4"); c != 402 || out["code"] != "keys_insufficient" {
		t.Fatalf("4th without Keys: %d %v", c, out)
	}
	f.balance["acc-ana"] = 7
	if c, _ := send("B4"); c != 202 || f.balance["acc-ana"] != 2 {
		t.Fatalf("Keys not spent: %d", f.balance["acc-ana"])
	}
	// The reply (either way) refunds in full; the sender never sees a decline.
	_, in := w.do("b4", "GET", "/v1/intents", "")
	id := in["inbound"].([]any)[0].(map[string]any)["id"].(string)
	if c, _ := w.do("b4", "POST", "/v1/intents/"+id+"/reply", `{"decision":"decline"}`); c != 204 || f.balance["acc-ana"] != 7 {
		t.Fatalf("refund on reply: %d", f.balance["acc-ana"])
	}
	_, mine := w.do("ana", "GET", "/v1/intents", "")
	for _, o := range mine["outbound"].([]any) {
		if o.(map[string]any)["state"] == "declined" {
			t.Fatal("decline shown to the sender")
		}
	}
	// TRYST+ is unlimited.
	f.plans["acc-ana"] = "tryst_plus"
	if c, out := send("B5"); c != 202 || out["state"] != "sent" || f.balance["acc-ana"] != 7 {
		t.Fatalf("TRYST+: %d %v", c, out)
	}
}

func TestInboundCapQueuesSilently(t *testing.T) {
	var names []string
	for i := 0; i < defaultInboxCap+2; i++ {
		names = append(names, fmt.Sprintf("m%02d", i))
	}
	w, f := introWorld(t, append(names, "ana")...)
	w.ready("ana", "Islington")
	w.intake("ana", "woman", `"men"`, `"ongoing"`, ``)
	for _, n := range names {
		w.ready(n, "Camden")
		w.intake(n, "man", `"women"`, `"ongoing"`, ``)
		f.plans["acc-"+n] = "tryst_plus"
		h := w.slate(n)["Ana"]
		if c, out := w.do(n, "POST", "/v1/intent", `{"target":"`+h+`"}`); c != 202 || out["state"] != "sent" {
			t.Fatalf("%s: %d %v", n, c, out)
		}
	}
	_, in := w.do("ana", "GET", "/v1/intents", "")
	if got := len(in["inbound"].([]any)); got != defaultInboxCap {
		t.Fatalf("inbox shows %d, cap %d", got, defaultInboxCap)
	}
	w.now = w.now.Add(25 * time.Hour)
	_, in = w.do("ana", "GET", "/v1/intents", "")
	if got := len(in["inbound"].([]any)); got != defaultInboxCap+2 {
		t.Fatalf("queued not delivered later: %d", got)
	}
}

func TestMutualIntentHandshakeOpensThreadOnlyWhenBothAccept(t *testing.T) {
	w, _ := introWorld(t, "ana", "ben")
	w.ready("ana", "Islington")
	w.ready("ben", "Camden")
	w.intake("ana", "woman", `"men"`, `"ongoing"`, ``)
	w.intake("ben", "man", `"women"`, `"ongoing"`, ``)
	w.do("ana", "POST", "/v1/intent", `{"target":"`+w.slate("ana")["Ben"]+`"}`)
	_, out := w.do("ben", "POST", "/v1/intent", `{"target":"`+w.slate("ben")["Ana"]+`"}`)
	hs, _ := out["handshake"].(string)
	if out["state"] != "mutual" || hs == "" {
		t.Fatalf("mutual: %v", out)
	}
	_, brief := w.do("ana", "GET", "/v1/handshake/"+hs+"/brief", "")
	b := brief["brief"].(map[string]any)
	if b["display_name"] != "Ben" || strings.Contains(fmt.Sprint(brief), "acc-") || brief["thread_id"] != nil {
		t.Fatalf("brief: %v", brief)
	}
	if c, _ := w.do("eve", "GET", "/v1/handshake/"+hs+"/brief", ""); c == 200 {
		t.Fatal("outsider read a brief")
	}
	_, d := w.do("ana", "POST", "/v1/handshake/"+hs+"/decision", `{"decision":"accept"}`)
	if d["state"] != "open" || len(w.s.th.threads) != 0 {
		t.Fatal("thread opened on one accept")
	}
	_, d = w.do("ben", "POST", "/v1/handshake/"+hs+"/decision", `{"decision":"accept"}`)
	if d["state"] != "accepted" || d["thread_id"] == "" || len(w.s.th.threads) != 1 {
		t.Fatalf("both accepted: %v", d)
	}
}

func TestEnvoyProposesButNeverSendsAlone(t *testing.T) {
	w, f := introWorld(t, "ana", "ben", "cy")
	w.ready("ana", "Islington")
	w.ready("ben", "Camden")
	w.ready("cy", "Hackney")
	w.intake("ana", "woman", `"men"`, `"ongoing"`, ``)
	w.intake("ben", "man", `"women"`, `"ongoing"`, ``)
	w.intake("cy", "man", `"women"`, `"ongoing"`, ``)
	if c, _ := w.do("ana", "GET", "/v1/envoy/proposals", ""); c != 403 {
		t.Fatalf("without ENVOY: %d", c)
	}
	f.plans["acc-ana"] = "envoy"
	_, out := w.do("ana", "GET", "/v1/envoy/proposals", "")
	props := out["proposals"].([]any)
	if len(props) != 2 || len(w.s.in.intents) != 0 {
		t.Fatalf("proposals: %v, intents %d", props, len(w.s.in.intents))
	}
	h := props[0].(map[string]any)["card"].(map[string]any)["handle"].(string)
	if c, out := w.do("ana", "POST", "/v1/envoy/proposals/"+h+"/approve", ""); c != 202 || out["state"] != "sent" {
		t.Fatalf("approve: %d %v", c, out)
	}
	for _, i := range w.s.in.intents {
		if i.Via != "envoy" {
			t.Fatal("via")
		}
	}
}

func TestCouplesCosignVetoAndDissolve(t *testing.T) {
	w, _ := introWorld(t, "ana", "ben", "cy")
	w.ready("ana", "Islington")
	w.ready("ben", "Camden")
	w.ready("cy", "Hackney")
	w.intake("ana", "woman", `"men","women"`, `"third"`, ``)
	w.intake("ben", "man", `"women"`, `"third"`, ``)
	w.intake("cy", "woman", `"couples"`, `"third"`, ``)

	_, inv := w.do("ana", "POST", "/v1/couple/invite", `{"device_id":"device-ana-1"}`)
	tok := inv["token"].(string)
	if c, out := w.do("ben", "POST", "/v1/couple/cosign", `{"token":"`+tok+`","device_id":"device-ana-1","approved_version":1}`); c != 409 || out["code"] != "same_device_cosign" {
		t.Fatalf("same device: %d %v", c, out)
	}
	if c, out := w.do("ben", "POST", "/v1/couple/cosign", `{"token":"`+tok+`","device_id":"device-ben-1","approved_version":2}`); c != 409 || out["code"] != "version_changed" {
		t.Fatalf("version: %d %v", c, out)
	}
	c, out := w.do("ben", "POST", "/v1/couple/cosign", `{"token":"`+tok+`","device_id":"device-ben-1","approved_version":1}`)
	if c != 201 {
		t.Fatalf("cosign: %d %v", c, out)
	}
	cp := out["couple"].(map[string]any)
	id := cp["id"].(string)
	if cp["partner"] != "Ana" || cp["veto_mode"] != "either" {
		t.Fatalf("couple: %v", cp)
	}
	// Partners are never introduced to each other; Cy (seeking couples) now sees both.
	if _, ok := w.slate("ana")["Ben"]; ok {
		t.Fatal("partners introduced")
	}
	if s := w.slate("cy"); s["Ana"] == "" || s["Ben"] == "" {
		t.Fatalf("Cy's slate: %v", s)
	}
	// Veto mode changes only when both ask.
	_, v := w.do("ana", "PUT", "/v1/couple/"+id+"/veto-mode", `{"veto_mode":"both"}`)
	if v["couple"].(map[string]any)["veto_mode"] != "either" {
		t.Fatal("one partner changed the veto mode")
	}
	_, v = w.do("ben", "PUT", "/v1/couple/"+id+"/veto-mode", `{"veto_mode":"both"}`)
	if v["couple"].(map[string]any)["veto_mode"] != "both" {
		t.Fatal("both asked")
	}
	// A handshake with the couple needs both partners' accept under "both".
	w.do("cy", "POST", "/v1/intent", `{"target":"`+w.slate("cy")["Ana"]+`"}`)
	_, out = w.do("ana", "POST", "/v1/intent", `{"target":"`+w.slate("ana")["Cy"]+`"}`)
	hs := out["handshake"].(string)
	w.do("cy", "POST", "/v1/handshake/"+hs+"/decision", `{"decision":"accept"}`)
	_, d := w.do("ana", "POST", "/v1/handshake/"+hs+"/decision", `{"decision":"accept"}`)
	if d["state"] != "open" {
		t.Fatal("opened without Ben")
	}
	_, d = w.do("ben", "POST", "/v1/handshake/"+hs+"/decision", `{"decision":"accept"}`)
	if d["state"] != "accepted" {
		t.Fatalf("Ben accepted too: %v", d)
	}
	// Either partner dissolves at once.
	w.do("ben", "DELETE", "/v1/couple/"+id, "")
	if _, out := w.do("ana", "GET", "/v1/couple", ""); out["couple"] != nil {
		t.Fatal("still linked")
	}
	// A couple that hasn't reconfirmed leaves discovery.
	_, inv = w.do("ana", "POST", "/v1/couple/invite", `{"device_id":"device-ana-1"}`)
	w.do("ben", "POST", "/v1/couple/cosign", `{"token":"`+inv["token"].(string)+`","device_id":"device-ben-1","approved_version":1}`)
	w.now = w.now.Add(reconfirmEvery + reconfirmGrace + time.Hour)
	if s := w.slate("cy"); s["Ana"] != "" {
		t.Fatal("lapsed couple still shown")
	}
}
