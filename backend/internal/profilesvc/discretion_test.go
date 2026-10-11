package profilesvc

import (
	"strings"
	"testing"
)

// stepUp marks a member's session stepped up (multiAuth stand-in).
func (w *world) stepUp(who string, on bool) {
	a := w.s.auth.(multiAuth)[who]
	a.StepUp = on
	w.s.auth.(multiAuth)[who] = a
}

func area(district string) string {
	for _, a := range Areas() {
		if a.District == district {
			return a.ID
		}
	}
	return ""
}

func (w *world) sees(viewer, name string) bool {
	_, out := w.do(viewer, "GET", "/v1/tonight", "")
	for _, p := range people(out, "people") {
		if p["display_name"] == name {
			return true
		}
	}
	return false
}

func (w *world) on(who ...string) {
	for _, m := range who {
		if c, out := w.do(m, "PUT", "/v1/tonight", `{"hours":3,"radius_km":25}`); c != 200 {
			w.t.Fatalf("%s on: %d %v", m, c, out)
		}
	}
}

func TestPauseVanishAndResume(t *testing.T) {
	w := newWorld(t, "dev", "ana", "ben")
	w.ready("ana", "Islington")
	w.ready("ben", "Camden")
	w.on("ana", "ben")
	if !w.sees("ana", "Ben") {
		t.Fatal("setup")
	}
	if c, out := w.do("ben", "POST", "/v1/account/pause", ""); c != 200 || out["status"] != "paused" {
		t.Fatalf("pause: %d %v", c, out)
	}
	if w.sees("ana", "Ben") {
		t.Fatal("paused member still shown")
	}
	if c, _ := w.do("ben", "PUT", "/v1/tonight", `{"hours":3,"radius_km":25}`); c != 409 {
		t.Fatalf("paused member started Tonight: %d", c)
	}
	if c, _ := w.do("ben", "POST", "/v1/account/resume", ""); c != 403 {
		t.Fatalf("resume without step-up: %d", c)
	}
	w.stepUp("ben", true)
	w.do("ben", "POST", "/v1/account/resume", "")
	w.on("ben")
	if !w.sees("ana", "Ben") {
		t.Fatal("resumed member not shown")
	}

	// Vanish closes conversations as well.
	w.s.mu.Lock()
	th := w.s.ensureThread(t.Context(), "acc-ana", "acc-ben", "tonight")
	w.s.mu.Unlock()
	w.do("ben", "POST", "/v1/account/vanish", "")
	if th.State != "closed" || w.sees("ana", "Ben") {
		t.Fatalf("vanish: thread %s", th.State)
	}
	if c, _ := w.do("ana", "GET", "/v1/threads/"+th.ID+"/photo", ""); c != 404 {
		t.Fatalf("vanished member's photo still served: %d", c)
	}
}

func TestPolicyBroadeningNeedsStepUp(t *testing.T) {
	w := newWorld(t, "dev", "ana", "ben", "cy")
	w.ready("ana", "Islington")
	w.ready("ben", "Camden")
	w.ready("cy", "Hackney")
	// Tightening never needs step-up.
	if c, out := w.do("ben", "PUT", "/v1/discretion/policy", `{"visibility":"incognito","max_distance_km":10,"min_counterparty_tier":"V2"}`); c != 200 || out["versions"].(float64) != 1 {
		t.Fatalf("tighten: %d %v", c, out)
	}
	w.on("ana", "ben", "cy")
	// Incognito: Ben is shown only to people he's interested in.
	if w.sees("ana", "Ben") || w.sees("cy", "Ben") {
		t.Fatal("incognito member shown")
	}
	_, out := w.do("ben", "GET", "/v1/tonight", "")
	for _, p := range people(out, "people") {
		if p["display_name"] == "Ana" {
			w.do("ben", "POST", "/v1/tonight/interest/"+p["handle"].(string), "")
		}
	}
	if !w.sees("ana", "Ben") || w.sees("cy", "Ben") {
		t.Fatal("incognito: only Ana should see Ben now")
	}
	// Broadening needs a fresh step-up.
	if c, _ := w.do("ben", "PUT", "/v1/discretion/policy", `{"visibility":"visible","max_distance_km":10,"min_counterparty_tier":"V2"}`); c != 403 {
		t.Fatalf("broaden without step-up: %d", c)
	}
	w.stepUp("ben", true)
	if c, _ := w.do("ben", "PUT", "/v1/discretion/policy", `{"visibility":"visible","max_distance_km":50,"min_counterparty_tier":"V2"}`); c != 200 {
		t.Fatalf("broaden with step-up: %d", c)
	}
	if !w.sees("cy", "Ben") {
		t.Fatal("visible again")
	}
	// A counterpart tier above Ana's hides both ways.
	w.do("ben", "PUT", "/v1/discretion/policy", `{"visibility":"visible","max_distance_km":50,"min_counterparty_tier":"V3"}`)
	if w.sees("ana", "Ben") || w.sees("ben", "Ana") {
		t.Fatal("tier floor not applied both ways")
	}
}

func TestExcludedAreasAndPartnerShield(t *testing.T) {
	w := newWorld(t, "dev", "ana", "ben", "cy")
	keys := map[string]string{"acc-ana": "k-ana", "acc-ben": "k-ben", "acc-cy": "k-cy"}
	w.s.ShieldKeyOf = func(a string) string { return keys[a] }
	w.s.BlindShield = func(d string) string {
		if len(d) != 64 {
			return ""
		}
		return "k-" + d[:3] // test stand-in: the digest's first letters name the member
	}
	w.ready("ana", "Islington")
	w.ready("ben", "Camden")
	w.ready("cy", "Hackney")
	w.on("ana", "ben", "cy")

	// Ana hides Camden: Ben (in Camden) and Ana no longer see each other, both ways.
	if c, _ := w.do("ana", "PUT", "/v1/discretion/zones", `{"zones":["`+area("Camden")+`"]}`); c != 200 {
		t.Fatalf("zones: %d", c)
	}
	if w.sees("ana", "Ben") || w.sees("ben", "Ana") || !w.sees("ana", "Cy") {
		t.Fatal("excluded area not applied both ways")
	}
	if c, _ := w.do("ana", "PUT", "/v1/discretion/zones", `{"zones":[]}`); c != 403 {
		t.Fatalf("removing a zone without step-up: %d", c)
	}

	// Cy shields a contact that turns out to be Ana's: silent, both ways.
	digest := "ana" + strings.Repeat("0", 61)
	if c, out := w.do("cy", "POST", "/v1/discretion/shield", `{"add":["`+digest+`"]}`); c != 200 || out["shield"].(float64) != 1 {
		t.Fatalf("shield: %d %v", c, out)
	}
	if w.sees("ana", "Cy") || w.sees("cy", "Ana") {
		t.Fatal("shielded pair still see each other")
	}
	if c, _ := w.do("cy", "DELETE", "/v1/discretion/shield", ""); c != 403 {
		t.Fatalf("clearing the shield without step-up: %d", c)
	}
	_, out := w.do("cy", "GET", "/v1/discretion", "")
	if out["shield"].(float64) != 1 {
		t.Fatal("shield lost")
	}
}

func TestEmergencyLockHidesProfile(t *testing.T) {
	w := newWorld(t, "dev", "ana", "ben")
	w.ready("ana", "Islington")
	w.ready("ben", "Camden")
	w.on("ana", "ben")
	w.s.Lock("acc-ben", true)
	if w.sees("ana", "Ben") {
		t.Fatal("locked member shown")
	}
	w.s.Lock("acc-ben", false)
	_, out := w.do("ben", "GET", "/v1/discretion", "")
	if out["status"] != "paused" {
		t.Fatalf("after recovery the member comes back paused: %v", out)
	}
}
