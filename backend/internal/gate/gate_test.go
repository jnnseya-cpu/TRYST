package gate

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var ok = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

func call(h http.Handler, c *Claims) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/v1/slate", nil)
	if c != nil {
		r = r.WithContext(WithClaims(r.Context(), *c))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// FR-001: discovery and every interactive surface is locked below V2.
func TestSlateLockedBelowV2(t *testing.T) {
	h := RequireTier(V2, ok)
	for _, tier := range []Tier{V0, V1} {
		w := call(h, &Claims{Tier: tier, Jurisdiction: "GB"})
		if w.Code != http.StatusForbidden || w.Header().Get("Content-Type") != "application/problem+json" {
			t.Fatalf("tier %s: got %d", tier, w.Code)
		}
	}
	for _, tier := range []Tier{V2, V3} {
		if w := call(h, &Claims{Tier: tier}); w.Code != http.StatusNoContent {
			t.Fatalf("tier %s: got %d", tier, w.Code)
		}
	}
}

func TestNoClaimsIsUnauthorised(t *testing.T) {
	if w := call(RequireTier(V0, ok), nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", w.Code)
	}
}

// FR-043: hard geo-block outside enabled markets.
func TestJurisdictionGate(t *testing.T) {
	h := Jurisdiction(Jurisdictions{"GB": true, "IE": true}, ok)
	if w := call(h, &Claims{Jurisdiction: "AE"}); w.Code != http.StatusForbidden {
		t.Fatalf("AE should be blocked, got %d", w.Code)
	}
	if w := call(h, &Claims{Jurisdiction: "IE"}); w.Code != http.StatusNoContent {
		t.Fatalf("IE should pass, got %d", w.Code)
	}
}

func TestArt9Consent(t *testing.T) {
	h := RequireArt9Consent(ok)
	if w := call(h, &Claims{}); w.Code != http.StatusForbidden {
		t.Fatalf("got %d", w.Code)
	}
	if w := call(h, &Claims{Art9Consent: true}); w.Code != http.StatusNoContent {
		t.Fatalf("got %d", w.Code)
	}
}
