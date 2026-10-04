// Command edge-gateway is the scaffold of the TRYST edge (docs/spec/03_Backend.md §2).
// It wires the gate middleware; token verification (OAuth2 + DPoP) is a stub until P1.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/gate"
	"github.com/jnnseya-cpu/tryst/backend/internal/problem"
)

func main() {
	allowed := gate.Jurisdictions{"GB": true, "IE": true}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	// Example gated route: discovery requires V2 (FR-001). Real handlers proxy to core-svc.
	mux.Handle("GET /v1/slate", gate.Jurisdiction(allowed, gate.RequireTier(gate.V2,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { problem.NotFound(w) }))))

	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("edge-gateway listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
