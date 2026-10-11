package profilesvc

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

const jwkA = `{"kty":"EC","crv":"P-256","x":"aaaa","y":"bbbb"}`
const jwkB = `{"kty":"EC","crv":"P-256","x":"cccc","y":"dddd"}`

// mutualPair returns a world where ana and ben matched in Tonight, and the thread id.
func mutualPair(t *testing.T) (*world, string) {
	w := newWorld(t, "dev", "ana", "ben", "eve")
	w.ready("ana", "Islington")
	w.ready("ben", "Camden")
	w.ready("eve", "Hackney")
	for _, n := range []string{"ana", "ben", "eve"} {
		w.do(n, "PUT", "/v1/tonight", `{"hours":4,"radius_km":10}`)
	}
	interest := func(who, name string) map[string]any {
		_, out := w.do(who, "GET", "/v1/tonight", "")
		for _, p := range people(out, "people") {
			if p["display_name"] == name {
				_, v := w.do(who, "POST", "/v1/tonight/interest/"+p["handle"].(string), "")
				return v
			}
		}
		t.Fatalf("%s can't see %s", who, name)
		return nil
	}
	interest("ana", "Ben")
	if _, out := w.do("ana", "GET", "/v1/threads", ""); len(out["threads"].([]any)) != 0 {
		t.Fatal("thread opened before interest was mutual")
	}
	v := interest("ben", "Ana")
	m := people(v, "mutual")
	if len(m) != 1 || m[0]["thread_id"] == nil {
		t.Fatalf("mutual without thread: %v", v)
	}
	return w, m[0]["thread_id"].(string)
}

func send(w *world, who, tid, device, client string, envs ...string) (int, map[string]any) {
	var parts []string
	for _, d := range envs {
		parts = append(parts, fmt.Sprintf(`{"device_id":"%s","epk":"e","iv":"i","ct":"ciphertext-for-%s"}`, d, d))
	}
	return w.do(who, "POST", "/v1/threads/"+tid+"/messages",
		fmt.Sprintf(`{"client_id":"%s","sender_device":"%s","envelopes":[%s]}`, client, device, strings.Join(parts, ",")))
}

func TestThreadOpensOnMutualAndRelaysCiphertextOnly(t *testing.T) {
	w, tid := mutualPair(t)
	// Device keys: public EC P-256 only.
	if c, _ := w.do("ana", "PUT", "/v1/devices/key", `{"device_id":"ana-phone-1","public_jwk":{"kty":"EC","crv":"P-256","x":"a","y":"b","d":"secret"}}`); c != 422 {
		t.Fatalf("private key accepted: %d", c)
	}
	w.do("ana", "PUT", "/v1/devices/key", `{"device_id":"ana-phone-1","public_jwk":`+jwkA+`}`)
	w.do("ben", "PUT", "/v1/devices/key", `{"device_id":"ben-phone-1","public_jwk":`+jwkB+`}`)
	_, th := w.do("ana", "GET", "/v1/threads/"+tid, "")
	if len(th["their_devices"].([]any)) != 1 || th["thread"].(map[string]any)["name"] != "Ben" {
		t.Fatalf("thread view: %v", th)
	}
	// Eve is not in the thread.
	if c, _ := w.do("eve", "GET", "/v1/threads/"+tid, ""); c != 404 {
		t.Fatalf("outsider saw the thread: %d", c)
	}
	if c, _ := send(w, "eve", tid, "x", "c1", "ben-phone-1"); c != 404 {
		t.Fatalf("outsider sent: %d", c)
	}
	// Envelopes only for this thread's devices; sender must use their own device.
	if c, _ := send(w, "ana", tid, "ana-phone-1", "c1", "ben-phone-1", "eve-device"); c != 422 {
		t.Fatalf("foreign device envelope: %d", c)
	}
	if c, _ := send(w, "ana", tid, "ben-phone-1", "c1", "ben-phone-1"); c != 422 {
		t.Fatalf("sending as someone else's device: %d", c)
	}
	c, out := send(w, "ana", tid, "ana-phone-1", "c1", "ben-phone-1", "ana-phone-1")
	if c != 201 || out["seq"].(float64) != 1 {
		t.Fatalf("send: %d %v", c, out)
	}
	// Retry with the same client id: no duplicate.
	if c, _ := send(w, "ana", tid, "ana-phone-1", "c1", "ben-phone-1", "ana-phone-1"); c != 200 {
		t.Fatalf("retry: %d", c)
	}
	_, msgs := w.do("ben", "GET", "/v1/threads/"+tid+"/messages?device_id=ben-phone-1", "")
	list := msgs["messages"].([]any)
	if len(list) != 1 {
		t.Fatalf("messages: %v", list)
	}
	m := list[0].(map[string]any)
	if m["mine"] != false || m["envelope"].(map[string]any)["ct"] != "ciphertext-for-ben-phone-1" {
		t.Fatalf("ben gets only his envelope: %v", m)
	}
	// Ana's own device copy.
	_, mine := w.do("ana", "GET", "/v1/threads/"+tid+"/messages?device_id=ana-phone-1", "")
	if mine["messages"].([]any)[0].(map[string]any)["mine"] != true {
		t.Fatal("own message not marked")
	}
	if _, after := w.do("ben", "GET", "/v1/threads/"+tid+"/messages?device_id=ben-phone-1&after=1", ""); len(after["messages"].([]any)) != 0 {
		t.Fatal("after cursor")
	}
}

func TestRetentionShortestWinsAndMessagesExpire(t *testing.T) {
	w, tid := mutualPair(t)
	w.do("ana", "PUT", "/v1/devices/key", `{"device_id":"ana-phone-1","public_jwk":`+jwkA+`}`)
	w.do("ben", "PUT", "/v1/devices/key", `{"device_id":"ben-phone-1","public_jwk":`+jwkB+`}`)
	if c, _ := w.do("ben", "PUT", "/v1/threads/"+tid+"/retention", `{"days":14}`); c != 422 {
		t.Fatal("14 days accepted")
	}
	w.do("ana", "PUT", "/v1/threads/"+tid+"/retention", `{"days":90}`)
	_, out := w.do("ben", "PUT", "/v1/threads/"+tid+"/retention", `{"days":7}`)
	if out["thread"].(map[string]any)["retention_days"].(float64) != 7 {
		t.Fatalf("shortest wins: %v", out)
	}
	send(w, "ana", tid, "ana-phone-1", "c1", "ben-phone-1")
	w.now = w.now.Add(8 * 24 * time.Hour)
	if _, m := w.do("ben", "GET", "/v1/threads/"+tid+"/messages?device_id=ben-phone-1", ""); len(m["messages"].([]any)) != 0 {
		t.Fatal("expired message still served")
	}
	if docs, _ := w.s.st.All(t.Context(), "msg:"+tid); len(docs) != 0 {
		t.Fatal("expired message not purged")
	}
}

func TestBlockClosesForBothAndHidesEverywhere(t *testing.T) {
	w, tid := mutualPair(t)
	w.do("ana", "PUT", "/v1/devices/key", `{"device_id":"ana-phone-1","public_jwk":`+jwkA+`}`)
	w.do("ben", "PUT", "/v1/devices/key", `{"device_id":"ben-phone-1","public_jwk":`+jwkB+`}`)
	w.do("ben", "POST", "/v1/threads/"+tid+"/block", "")
	if c, _ := send(w, "ana", tid, "ana-phone-1", "c2", "ben-phone-1"); c != 409 {
		t.Fatalf("send after block: %d", c)
	}
	_, out := w.do("ana", "GET", "/v1/threads", "")
	if out["threads"].([]any)[0].(map[string]any)["state"] != "closed" {
		t.Fatal("ana should see the conversation closed")
	}
	_, tn := w.do("ana", "GET", "/v1/tonight", "")
	for _, p := range append(people(tn, "people"), people(tn, "mutual")...) {
		if p["display_name"] == "Ben" {
			t.Fatal("blocked member still visible in Tonight")
		}
	}
	if c, _ := w.do("ana", "GET", "/v1/threads/"+tid+"/photo", ""); c != 404 {
		t.Fatal("blocked member's photo served")
	}
}

func TestGuardianSignalFreezesWithoutContent(t *testing.T) {
	w, tid := mutualPair(t)
	var got []string
	w.s.OnSignal = func(th, reporter, subject, kind string) { got = append(got, kind) }
	if c, _ := w.do("ben", "POST", "/v1/threads/"+tid+"/signal", `{"kind":"gossip"}`); c != 422 {
		t.Fatal("unknown signal kind")
	}
	w.do("ben", "POST", "/v1/threads/"+tid+"/signal", `{"kind":"fraud","model":"baseline-v1","about":"received"}`)
	_, out := w.do("ben", "GET", "/v1/threads/"+tid, "")
	if out["thread"].(map[string]any)["state"] != "normal" {
		t.Fatal("fraud warns, does not freeze")
	}
	w.do("ben", "POST", "/v1/threads/"+tid+"/signal", `{"kind":"extortion","model":"baseline-v1","about":"received"}`)
	_, out = w.do("ana", "GET", "/v1/threads/"+tid, "")
	if out["thread"].(map[string]any)["state"] != "frozen" {
		t.Fatal("extortion must freeze")
	}
	if len(got) != 2 || got[1] != "extortion" {
		t.Fatalf("signals to safety queue: %v", got)
	}
}

func TestThreadsSurviveRestart(t *testing.T) {
	w, tid := mutualPair(t)
	w.do("ana", "PUT", "/v1/devices/key", `{"device_id":"ana-phone-1","public_jwk":`+jwkA+`}`)
	w.do("ben", "PUT", "/v1/devices/key", `{"device_id":"ben-phone-1","public_jwk":`+jwkB+`}`)
	send(w, "ana", tid, "ana-phone-1", "c1", "ben-phone-1")
	w.do("ben", "POST", "/v1/threads/"+tid+"/block", "")
	s2, err := NewWithStore(w.s.auth, DevModerator{}, "dev", w.s.st, w.s.vault)
	if err != nil {
		t.Fatal(err)
	}
	s2.now = w.s.now
	w.s = s2
	if _, m := w.do("ben", "GET", "/v1/threads/"+tid+"/messages?device_id=ben-phone-1", ""); len(m["messages"].([]any)) != 1 {
		t.Fatal("message lost on restart")
	}
	if w.s.isBlocked("acc-ben", "acc-ana") == false {
		t.Fatal("block lost on restart")
	}
}
