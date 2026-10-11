// Package metrics exposes operational metrics in the Prometheus text format (03 §12;
// FR-121; D-57). It is deliberately small and dependency-free so what it records can be
// audited: labels are route shapes, methods, status codes and event types, never member
// identifiers, and IDs in paths are collapsed to "{id}". Metrics are served on a separate
// internal port, never on the public listener.
package metrics

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Registry holds counters, histograms and gauges.
type Registry struct {
	mu       sync.Mutex
	help     map[string]string
	counters map[string]map[string]float64 // name -> label string -> value
	hists    map[string]map[string]*hist
	gauges   map[string]func() float64
}

type hist struct {
	buckets []float64
	counts  []uint64
	sum     float64
	n       uint64
}

// DefBuckets suit request latencies in seconds.
var DefBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}

func NewRegistry() *Registry {
	return &Registry{help: map[string]string{}, counters: map[string]map[string]float64{}, hists: map[string]map[string]*hist{}, gauges: map[string]func() float64{}}
}

// Default is the process-wide registry.
var Default = NewRegistry()

func labelString(kv []string) string {
	if len(kv) == 0 {
		return ""
	}
	parts := make([]string, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		parts = append(parts, kv[i]+`="`+strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(kv[i+1])+`"`)
	}
	sort.Strings(parts)
	return "{" + strings.Join(parts, ",") + "}"
}

// Help sets the HELP text for a metric.
func (r *Registry) Help(name, text string) {
	r.mu.Lock()
	r.help[name] = text
	r.mu.Unlock()
}

// Inc adds 1 to a counter. Labels are key, value pairs.
func (r *Registry) Inc(name string, labels ...string) { r.Add(name, 1, labels...) }

// Add adds v to a counter.
func (r *Registry) Add(name string, v float64, labels ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.counters[name]
	if !ok {
		m = map[string]float64{}
		r.counters[name] = m
	}
	m[labelString(labels)] += v
}

// Observe records a histogram sample.
func (r *Registry) Observe(name string, v float64, labels ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.hists[name]
	if !ok {
		m = map[string]*hist{}
		r.hists[name] = m
	}
	ls := labelString(labels)
	h, ok := m[ls]
	if !ok {
		h = &hist{buckets: DefBuckets, counts: make([]uint64, len(DefBuckets))}
		m[ls] = h
	}
	for i, b := range h.buckets {
		if v <= b {
			h.counts[i]++
		}
	}
	h.sum += v
	h.n++
}

// Gauge registers a function read at scrape time.
func (r *Registry) Gauge(name string, f func() float64) {
	r.mu.Lock()
	r.gauges[name] = f
	r.mu.Unlock()
}

func num(v float64) string {
	if math.IsInf(v, 1) {
		return "+Inf"
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// Write renders the text exposition format.
func (r *Registry) Write(w io.Writer) {
	r.mu.Lock()
	gauges := map[string]func() float64{}
	for k, v := range r.gauges {
		gauges[k] = v
	}
	r.mu.Unlock()
	gv := map[string]float64{}
	for k, f := range gauges { // outside the lock: gauges may take other locks
		gv[k] = f()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	help := func(name, typ string) {
		if h, ok := r.help[name]; ok {
			fmt.Fprintf(w, "# HELP %s %s\n", name, h)
		}
		fmt.Fprintf(w, "# TYPE %s %s\n", name, typ)
	}
	for _, name := range sortedKeys(r.counters) {
		help(name, "counter")
		for _, ls := range sortedKeys(r.counters[name]) {
			fmt.Fprintf(w, "%s%s %s\n", name, ls, num(r.counters[name][ls]))
		}
	}
	for _, name := range sortedKeys(r.hists) {
		help(name, "histogram")
		for _, ls := range sortedKeys(r.hists[name]) {
			h := r.hists[name][ls]
			inner := strings.TrimSuffix(strings.TrimPrefix(ls, "{"), "}")
			sep := ""
			if inner != "" {
				sep = ","
			}
			for i, b := range h.buckets {
				fmt.Fprintf(w, "%s_bucket{%s%sle=\"%s\"} %d\n", name, inner, sep, num(b), h.counts[i])
			}
			fmt.Fprintf(w, "%s_bucket{%s%sle=\"+Inf\"} %d\n", name, inner, sep, h.n)
			fmt.Fprintf(w, "%s_sum%s %s\n%s_count%s %d\n", name, ls, num(h.sum), name, ls, h.n)
		}
	}
	for _, name := range sortedKeys(gv) {
		help(name, "gauge")
		fmt.Fprintf(w, "%s %s\n", name, num(gv[name]))
	}
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// Handler serves the registry.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		r.Write(w)
	})
}

var idSeg = regexp.MustCompile(`^([a-z]{1,4}_[0-9a-zA-Z]{6,}|[0-9a-fA-F]{16,}|[0-9]+|[A-Za-z0-9_-]{20,})$`)

// Route collapses a path to its shape: IDs become {id}, and only the first five segments
// count, so labels stay few and never carry identifiers.
func Route(path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	if len(segs) > 5 {
		segs = append(segs[:5], "…")
	}
	for i, s := range segs {
		if idSeg.MatchString(s) {
			segs[i] = "{id}"
		}
	}
	return "/" + strings.Join(segs, "/")
}

type recorder struct {
	http.ResponseWriter
	code int
}

func (r *recorder) WriteHeader(c int) {
	if r.code == 0 {
		r.code = c
	}
	r.ResponseWriter.WriteHeader(c)
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.code == 0 {
		r.code = 200
	}
	return r.ResponseWriter.Write(b)
}

// Middleware counts requests and latency by route shape.
func (r *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		rec := &recorder{ResponseWriter: w}
		next.ServeHTTP(rec, req)
		code := rec.code
		if code == 0 {
			code = 200
		}
		route := Route(req.URL.Path)
		r.Inc("tryst_http_requests_total", "route", route, "method", req.Method, "code", strconv.Itoa(code))
		r.Observe("tryst_http_request_duration_seconds", time.Since(start).Seconds(), "route", route)
	})
}
