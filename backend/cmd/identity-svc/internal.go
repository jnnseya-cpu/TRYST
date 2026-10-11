package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/notify"
	"github.com/jnnseya-cpu/tryst/backend/internal/profilesvc"
)

// internalAPI is the member platform's internal API for the safety service (D-59). It is
// served on the internal listener behind svcauth.Verifier and gives the safety service only
// what moderation needs: who a bearer token belongs to, who is on the other side of a
// thread, a pseudonym and city, the main photo (for a moderator who recorded why), and a
// way to apply standing and send masked notices.
func internalAPI(srv *identity.Server, prof *profilesvc.Service, notifier *notify.Service) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("POST /internal/v1/authenticate", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Authorization string `json:"authorization"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in)
		probe := httptest.NewRequest("GET", "/", nil)
		probe.Header.Set("Authorization", in.Authorization)
		acc, ok := srv.Authenticate(probe)
		if !ok {
			w.WriteHeader(401)
			return
		}
		writeInternal(w, map[string]any{"account": acc.ID, "tier": int(acc.Tier), "birth_ym": acc.BirthYM})
	})
	m.HandleFunc("GET /internal/v1/threads/{id}/counterpart", func(w http.ResponseWriter, r *http.Request) {
		other, ok := prof.Counterpart(r.URL.Query().Get("member"), r.PathValue("id"))
		if !ok {
			w.WriteHeader(404)
			return
		}
		writeInternal(w, map[string]string{"member": other})
	})
	m.HandleFunc("GET /internal/v1/members/{id}/summary", func(w http.ResponseWriter, r *http.Request) {
		name, area := prof.Summary(r.PathValue("id"))
		writeInternal(w, map[string]string{"name": name, "area": area})
	})
	m.HandleFunc("GET /internal/v1/members/{id}/main-photo", func(w http.ResponseWriter, r *http.Request) {
		data, ct, err := prof.MainPhoto(r.Context(), r.PathValue("id"))
		if err != nil {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data) // #nosec G705 -- internal API; our own re-encoded image
	})
	m.HandleFunc("PUT /internal/v1/members/{id}/standing", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Standing string `json:"standing"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&in)
		switch in.Standing {
		case "good", "warn", "shadow_limit", "suspend", "permanent":
		default:
			http.Error(w, "unknown standing", 400)
			return
		}
		prof.SetStanding(r.PathValue("id"), in.Standing)
		w.WriteHeader(204)
	})
	m.HandleFunc("POST /internal/v1/notices", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Account, Kind, Link string }
		_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in)
		if in.Account == "" {
			http.Error(w, "account required", 400)
			return
		}
		notifier.Notify(in.Account, in.Kind, in.Link)
		w.WriteHeader(204)
	})
	return m
}

func writeInternal(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
