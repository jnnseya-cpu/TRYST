package safetysvc

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/jnnseya-cpu/tryst/backend/internal/vendorkeys"
)

// Vendor keys in the staff console: a lead sees which vendor settings are missing, enters
// them, and applies them. Values are write-only: they go straight to the store and are never
// returned, logged or put in the audit chain (which records who changed which setting, and
// when). Moderators can't see this page at all.

func (s *Service) leadOr403(w http.ResponseWriter, r *http.Request) (Staff, bool) {
	st, ok := s.staffOr401(w, r)
	if !ok {
		return Staff{}, false
	}
	if !st.Has("lead") {
		fail(w, 403, "lead_required", "Only a safety lead can manage vendor keys.")
		return Staff{}, false
	}
	if s.Settings == nil {
		fail(w, 503, "settings_unavailable", "Vendor keys can't be managed on this server.")
		return Staff{}, false
	}
	return st, true
}

type settingView struct {
	vendorkeys.Setting
	vendorkeys.Status
}

type groupView struct {
	vendorkeys.Group
	Ready    bool          `json:"ready"`
	Missing  int           `json:"missing"`
	Settings []settingView `json:"settings"`
}

func (s *Service) handleVendorKeys(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.leadOr403(w, r); !ok {
		return
	}
	status, err := s.Settings.Status(r.Context(), vendorkeys.Names())
	if err != nil {
		log.Printf("vendor keys status: %v", err)
		fail(w, 502, "settings_unavailable", "We couldn't read the key store. Try again in a minute.")
		return
	}
	var groups []groupView
	launchReady := true
	for _, g := range vendorkeys.Groups {
		gv := groupView{Group: g, Ready: true}
		for _, st := range vendorkeys.Catalogue {
			if st.Group != g.Key {
				continue
			}
			v := settingView{Setting: st, Status: status[st.Name]}
			if !v.Set {
				gv.Missing++
				gv.Ready = false
			}
			gv.Settings = append(gv.Settings, v)
		}
		if g.Required && !gv.Ready {
			launchReady = false
		}
		groups = append(groups, gv)
	}
	writeJSON(w, 200, map[string]any{"groups": groups, "launch_ready": launchReady})
}

func (s *Service) handleVendorKeySet(w http.ResponseWriter, r *http.Request) {
	st, ok := s.leadOr403(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	var in struct {
		Value string `json:"value"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&in) != nil {
		fail(w, 422, "validation_failed", "Enter a value.")
		return
	}
	v, err := vendorkeys.Validate(name, in.Value)
	if errors.Is(err, vendorkeys.ErrUnknown) {
		fail(w, 404, "not_found", "Not found")
		return
	}
	if err != nil {
		fail(w, 422, "validation_failed", err.Error())
		return
	}
	if err := s.Settings.Set(r.Context(), name, v); err != nil {
		log.Printf("vendor key save failed: %v", err)
		fail(w, 502, "settings_unavailable", "We couldn't save it. Nothing changed; try again in a minute.")
		return
	}
	s.mu.Lock()
	s.audit(r.Context(), st.ID, "vendor_key_set", "", name)
	s.mu.Unlock()
	status, _ := s.Settings.Status(r.Context(), []string{name})
	writeJSON(w, 200, map[string]any{"name": name, "set": true, "updated": status[name].Updated,
		"message": "Saved. Press Apply when you've entered everything you have."})
}

func (s *Service) handleVendorKeysApply(w http.ResponseWriter, r *http.Request) {
	st, ok := s.leadOr403(w, r)
	if !ok {
		return
	}
	if err := s.Settings.Apply(r.Context()); err != nil {
		log.Printf("vendor keys apply: %v", err)
		fail(w, 502, "settings_unavailable", "We couldn't apply the keys. Nothing changed; try again in a minute.")
		return
	}
	s.mu.Lock()
	s.audit(r.Context(), st.ID, "vendor_keys_apply", "", "")
	s.mu.Unlock()
	writeJSON(w, 202, map[string]any{"message": "Applying. Within about two minutes the app restarts with the new keys; it's offline for around 30 seconds."})
}
