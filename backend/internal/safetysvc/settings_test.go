package safetysvc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/store"
	"github.com/jnnseya-cpu/tryst/backend/internal/vendorkeys"
)

// Vendor keys: leads only, checked before storing, write-only, and audited by name only.
func TestVendorKeysAreLeadOnlyWriteOnlyAndAudited(t *testing.T) {
	g := newRig(t, store.NewMemory())
	keys := &vendorkeys.Memory{}
	g.svc.Settings = keys

	if w, _ := g.do("GET", "/ops/v1/settings/vendor-keys", "", tokMod1, nil); w.Code != 403 {
		t.Fatalf("moderator saw vendor keys: %d", w.Code)
	}
	if w, _ := g.do("PUT", "/ops/v1/settings/vendor-keys/ARC_SECRET", "", tokMod1, map[string]any{"value": "a-moderator-secret-123"}); w.Code != 403 {
		t.Fatalf("moderator set a key: %d", w.Code)
	}

	_, out := g.do("GET", "/ops/v1/settings/vendor-keys", "", tokLead, nil)
	if out["launch_ready"] != false {
		t.Fatal("ready with nothing set")
	}

	// Checked before storing: nothing invalid ever reaches the store.
	for name, bad := range map[string]string{"ARC_URL": "http://arc.example", "STRIPE_SECRET_KEY": "pk_live_publishable",
		"ASSURANCE_CALLBACK_SECRET": "too-short", "OTP_EMAIL_FROM": "TRYST <hello@example.org>", "ASSURANCE_CERTS": `["yoti"]`} {
		if w, _ := g.do("PUT", "/ops/v1/settings/vendor-keys/"+name, "", tokLead, map[string]any{"value": bad}); w.Code != 422 {
			t.Fatalf("%s accepted %q: %d", name, bad, w.Code)
		}
	}
	if w, _ := g.do("PUT", "/ops/v1/settings/vendor-keys/DATA_KEK", "", tokLead, map[string]any{"value": "00112233445566778899aabbccddeeff"}); w.Code != 404 {
		t.Fatalf("internal key settable from the console: %d", w.Code)
	}
	if len(keys.Values) != 0 {
		t.Fatalf("invalid values stored: %v", keys.Values)
	}

	const secret = "arc-signing-secret-do-not-show"
	w, _ := g.do("PUT", "/ops/v1/settings/vendor-keys/ARC_SECRET", "", tokLead, map[string]any{"value": "  " + secret + "\n"})
	if w.Code != 200 || keys.Values["ARC_SECRET"] != secret {
		t.Fatalf("set: %d %q", w.Code, keys.Values["ARC_SECRET"])
	}
	w, _ = g.do("GET", "/ops/v1/settings/vendor-keys", "", tokLead, nil)
	if strings.Contains(w.Body.String(), secret) {
		t.Fatal("a stored value was returned")
	}
	var view struct {
		Groups []struct {
			Key      string `json:"key"`
			Settings []struct {
				Name string `json:"name"`
				Set  bool   `json:"set"`
			} `json:"settings"`
		} `json:"groups"`
	}
	json.Unmarshal(w.Body.Bytes(), &view)
	found := false
	for _, gr := range view.Groups {
		for _, st := range gr.Settings {
			if st.Name == "ARC_SECRET" && st.Set {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("ARC_SECRET not shown as set")
	}

	if w, _ := g.do("POST", "/ops/v1/settings/vendor-keys/apply", "", tokLead, nil); w.Code != 202 || keys.Applied != 1 {
		t.Fatalf("apply: %d applied=%d", w.Code, keys.Applied)
	}
	var setEntry, applyEntry bool
	for _, e := range g.svc.chain {
		if strings.Contains(e.Detail, secret) || strings.Contains(e.Case, secret) {
			t.Fatal("a value reached the audit log")
		}
		setEntry = setEntry || (e.Action == "vendor_key_set" && e.Detail == "ARC_SECRET" && e.Staff == "lead1")
		applyEntry = applyEntry || (e.Action == "vendor_keys_apply" && e.Staff == "lead1")
	}
	if !setEntry || !applyEntry {
		t.Fatal("change or apply not audited")
	}
}
