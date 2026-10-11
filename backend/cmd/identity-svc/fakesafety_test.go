package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/svcauth"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcenv"
)

// fakeSafety stands in for cmd/safety-svc's internal API, checking signatures with the
// dev secret, and records what it receives.
type fakeSafety struct {
	mu      sync.Mutex
	signals []map[string]string
	erased  []string
	down    bool
}

func startFakeSafety(t *testing.T) (*fakeSafety, string) {
	f := &fakeSafety{}
	m := http.NewServeMux()
	m.HandleFunc("GET /internal/v1/ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	m.HandleFunc("POST /internal/v1/signals", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.down {
			w.WriteHeader(503)
			return
		}
		var in map[string]string
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &in)
		f.signals = append(f.signals, in)
		w.WriteHeader(204)
	})
	m.HandleFunc("POST /internal/v1/erase", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		f.erased = append(f.erased, in["account"])
		f.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"part": "safety", "erased": 0})
	})
	m.HandleFunc("POST /internal/v1/export", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"standing": "good"})
	})
	v := &svcauth.Verifier{Secrets: map[string][]byte{"profile": svcenv.InternalSecret("dev", "INTERNAL_SECRET_SAFETY")}}
	srv := httptest.NewServer(v.Wrap(m))
	t.Cleanup(srv.Close)
	t.Setenv("SAFETY_INTERNAL_URL", srv.URL)
	return f, srv.URL
}
