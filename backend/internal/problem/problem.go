// Package problem writes RFC 9457 problem+json responses (docs/spec/02_Shared_Contracts.md §5.10).
package problem

import (
	"encoding/json"
	"net/http"
)

// Problem is the wire shape. Code is the stable machine-readable value from the contract.
type Problem struct {
	Type         string `json:"type"`
	Title        string `json:"title"`
	Status       int    `json:"status"`
	Code         string `json:"code"`
	RequiredTier string `json:"required_tier,omitempty"`
	Factor       string `json:"factor,omitempty"`
}

// Write sends p with the right content type and status.
func Write(w http.ResponseWriter, p Problem) {
	if p.Type == "" {
		p.Type = "about:blank"
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// NotFound is the single response for anything the caller may not see, whatever the
// real reason (block, ExclusionRing, policy, jurisdiction). Existence never leaks.
func NotFound(w http.ResponseWriter) {
	Write(w, Problem{Title: "Not found", Status: http.StatusNotFound, Code: "not_found"})
}
