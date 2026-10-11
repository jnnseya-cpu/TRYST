package dsr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

type fakePart struct {
	name   string
	log    *[]string
	fails  int // fail this many times before succeeding
	data   map[string]string
	retain string
}

func (f *fakePart) Name() string { return f.name }
func (f *fakePart) Export(_ context.Context, a string) (any, error) {
	if v, ok := f.data[a]; ok {
		return map[string]string{"value": v}, nil
	}
	return nil, nil
}
func (f *fakePart) Erase(_ context.Context, a string) (Receipt, error) {
	if f.fails > 0 {
		f.fails--
		return Receipt{}, errors.New("temporarily unavailable")
	}
	*f.log = append(*f.log, f.name)
	delete(f.data, a)
	rc := Receipt{Erased: 1}
	if f.retain != "" {
		rc.Retained = []string{f.retain}
	}
	return rc, nil
}

func auth(r *http.Request) (string, bool) {
	id := r.Header.Get("X-Test-Member")
	return id, id != ""
}

func seed() []byte { return bytes.Repeat([]byte{7}, 32) }

func call(s *Service, method, path, who string, body any) (int, map[string]any) {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	if who != "" {
		r.Header.Set("X-Test-Member", who)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestExportAndDeleteWithSignedProof(t *testing.T) {
	var order []string
	parts := []Part{
		&fakePart{name: "profile", log: &order, data: map[string]string{"ana": "photos"}},
		&fakePart{name: "safety", log: &order, data: map[string]string{}, retain: "open case"},
		&fakePart{name: "identity", log: &order, data: map[string]string{"ana": "passkeys"}},
	}
	st := store.NewMemory()
	s, err := New(auth, st, seed(), parts...)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(auth, st, []byte("short")); err == nil {
		t.Fatal("short signing key accepted")
	}

	r := httptest.NewRequest("POST", "/v1/account/export", nil)
	r.Header.Set("X-Test-Member", "ana")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") ||
		!strings.Contains(w.Body.String(), "photos") || !strings.Contains(w.Body.String(), "passkeys") {
		t.Fatalf("export: %d %s", w.Code, w.Body)
	}
	if c, _ := call(s, "POST", "/v1/account/export", "", nil); c != 401 {
		t.Fatalf("export signed out: %d", c)
	}

	if c, _ := call(s, "DELETE", "/v1/account", "ana", map[string]string{"confirm": "yes"}); c != 422 {
		t.Fatalf("delete without the phrase: %d", c)
	}
	if len(order) != 0 {
		t.Fatal("erased without confirmation")
	}
	c, out := call(s, "DELETE", "/v1/account", "ana", map[string]string{"confirm": "Delete my account"})
	if c != 200 || out["status"] != "complete" {
		t.Fatalf("delete: %d %v", c, out)
	}
	if strings.Join(order, ",") != "profile,safety,identity" {
		t.Fatalf("erasure order: %v", order)
	}
	raw, _ := json.Marshal(out["completion_proof"])
	var p Proof
	_ = json.Unmarshal(raw, &p)
	if !Verify(s.PublicKey(), p) || p.Subject != SubjectCode("ana") || strings.Contains(string(raw), `"ana"`) {
		t.Fatalf("proof: %s", raw)
	}
	if len(p.Receipts) != 3 || p.Receipts[1].Retained[0] != "open case" {
		t.Fatalf("receipts: %+v", p.Receipts)
	}
	p.Receipts[1].Retained = nil
	if Verify(s.PublicKey(), p) {
		t.Fatal("tampered proof verified")
	}
	// The proof can be fetched later by its id, without signing in; the job is gone.
	if c, got := call(s, "GET", "/v1/deletion-proofs/"+p.JobID, "", nil); c != 200 || got["signature"] == nil {
		t.Fatalf("proof fetch: %d %v", c, got)
	}
	if c, k := call(s, "GET", "/v1/deletion-proofs/key", "", nil); c != 200 || k["alg"] != "Ed25519" {
		t.Fatalf("key: %d %v", c, k)
	}
	if jobs, _ := st.All(context.Background(), "job"); len(jobs) != 0 {
		t.Fatal("job with the account id left behind")
	}
}

func TestInterruptedErasureIsFinishedBySweep(t *testing.T) {
	var order []string
	commerce := &fakePart{name: "commerce", log: &order, fails: 1, data: map[string]string{}}
	parts := []Part{&fakePart{name: "profile", log: &order, data: map[string]string{}}, commerce, &fakePart{name: "identity", log: &order, data: map[string]string{}}}
	st := store.NewMemory()
	s, _ := New(auth, st, seed(), parts...)
	c, out := call(s, "DELETE", "/v1/account", "ben", map[string]string{"confirm": "delete my account"})
	if c != 202 || out["status"] != "in_progress" {
		t.Fatalf("interrupted: %d %v", c, out)
	}
	if strings.Join(order, ",") != "profile" {
		t.Fatalf("ran past the failure: %v", order)
	}
	// A fresh process (restart) picks the job up from the store and finishes it.
	s2, _ := New(auth, st, seed(), parts...)
	s2.Sweep(context.Background())
	if strings.Join(order, ",") != "profile,commerce,identity" {
		t.Fatalf("after sweep: %v", order)
	}
	id := out["deletion_job_id"].(string)
	if c, got := call(s2, "GET", "/v1/deletion-proofs/"+id, "", nil); c != 200 || len(got["receipts"].([]any)) != 3 {
		t.Fatalf("proof after sweep: %d %v", c, got)
	}
}
