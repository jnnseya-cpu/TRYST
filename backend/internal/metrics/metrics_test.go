package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouteCollapsesIdentifiers(t *testing.T) {
	cases := map[string]string{
		"/v1/threads/th_0123456789abcdef01234567/messages": "/v1/threads/{id}/messages",
		"/ops/v1/cases/cs_1cecaaf4d78fe74106":              "/ops/v1/cases/{id}",
		"/v1/tonight/photos/AbCdEfGhIjKlMnOpQrStUv/2":      "/v1/tonight/photos/{id}/{id}",
		"/v1/deletion-proofs/del_0123456789abcdef01234567": "/v1/deletion-proofs/{id}",
		"/v1/profile":       "/v1/profile",
		"/v1/a/b/c/d/e/f/g": "/v1/a/b/c/d/…",
	}
	for in, want := range cases {
		if got := Route(in); got != want {
			t.Errorf("Route(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExposition(t *testing.T) {
	r := NewRegistry()
	r.Help("tryst_http_requests_total", "Requests")
	h := r.Middleware(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "missing") {
			w.WriteHeader(404)
			return
		}
		w.Write([]byte("ok"))
	}))
	for _, p := range []string{"/v1/threads/th_0123456789abcdef01234567", "/v1/threads/th_aaaaaaaaaaaaaaaaaaaaaaaa", "/v1/missing"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", p, nil))
	}
	r.Gauge("tryst_safety_cases_overdue", func() float64 { return 2 })
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	out := w.Body.String()
	for _, want := range []string{
		"# HELP tryst_http_requests_total Requests",
		`tryst_http_requests_total{code="200",method="GET",route="/v1/threads/{id}"} 2`,
		`tryst_http_requests_total{code="404",method="GET",route="/v1/missing"} 1`,
		`tryst_http_request_duration_seconds_bucket{route="/v1/threads/{id}",le="+Inf"} 2`,
		`tryst_http_request_duration_seconds_count{route="/v1/threads/{id}"} 2`,
		"tryst_safety_cases_overdue 2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "th_0123") {
		t.Fatal("identifier leaked into labels")
	}
}
