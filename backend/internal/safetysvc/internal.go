package safetysvc

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

// InternalHandler is the safety service's internal API (D-59), for the member platform
// only. Mount it on the internal listener behind svcauth.Verifier:
//
//	POST /internal/v1/signals  {id, thread_id, reporter, subject, kind}  on-device signal
//	POST /internal/v1/export   {account}  data-rights export part
//	POST /internal/v1/erase    {account}  data-rights erasure part (idempotent)
func (s *Service) InternalHandler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /internal/v1/ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	m.HandleFunc("POST /internal/v1/signals", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID       string `json:"id"`
			Thread   string `json:"thread_id"`
			Reporter string `json:"reporter"`
			Subject  string `json:"subject"`
			Kind     string `json:"kind"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&in) != nil || in.ID == "" {
			http.Error(w, "bad signal", 400)
			return
		}
		if err := s.Signal(r.Context(), in.ID, in.Thread, in.Reporter, in.Subject, in.Kind); err != nil {
			log.Printf("signal: %v", err)
			http.Error(w, "retry", http.StatusServiceUnavailable)
			return
		}
		s.Flush(r.Context())
		w.WriteHeader(204)
	})
	m.HandleFunc("POST /internal/v1/export", func(w http.ResponseWriter, r *http.Request) {
		acc := accountFrom(w, r)
		if acc == "" {
			return
		}
		out, err := s.Export(r.Context(), acc)
		if err != nil {
			http.Error(w, "retry", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, 200, out)
	})
	m.HandleFunc("POST /internal/v1/erase", func(w http.ResponseWriter, r *http.Request) {
		acc := accountFrom(w, r)
		if acc == "" {
			return
		}
		rc, err := s.Erase(r.Context(), acc)
		if err != nil {
			http.Error(w, "retry", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, 200, rc)
	})
	return m
}

func accountFrom(w http.ResponseWriter, r *http.Request) string {
	var in struct {
		Account string `json:"account"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in) != nil || in.Account == "" {
		http.Error(w, "account required", 400)
		return ""
	}
	return in.Account
}
