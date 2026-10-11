package consent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

func members(r *http.Request) (string, bool) {
	a := r.Header.Get("X-Test-Member")
	return a, a != ""
}

func call(l *Ledger, method, who string, body any) (int, map[string]any) {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "/v1/consents", bytes.NewReader(b))
	if who != "" {
		r.Header.Set("X-Test-Member", who)
	}
	w := httptest.NewRecorder()
	l.ServeHTTP(w, r)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestLedgerIsAppendOnlyAndChained(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	l, err := New(ctx, st, members)
	if err != nil {
		t.Fatal(err)
	}
	var changes []string
	l.OnChange = func(a, k string, g bool) { changes = append(changes, k) }

	if c, _ := call(l, "POST", "", map[string]any{"kind": Art9, "granted": true}); c != 401 {
		t.Fatalf("signed out: %d", c)
	}
	// The conduct declaration is recorded where it's made, not here; unknown kinds refused.
	for _, k := range []string{ConductAttestation, "marketing"} {
		if c, _ := call(l, "POST", "ana", map[string]any{"kind": k, "granted": true}); c != 422 {
			t.Fatalf("%s accepted directly: %d", k, c)
		}
	}
	if c, out := call(l, "POST", "ana", map[string]any{"kind": Art9, "granted": true}); c != 201 || out["scope"] != Versions[Art9] {
		t.Fatalf("grant: %d %v", c, out)
	}
	if _, err := l.Append(ctx, "ana", ConductAttestation, true, "tos-3.5/2026-10"); err != nil {
		t.Fatal(err)
	}
	call(l, "POST", "ana", map[string]any{"kind": Art9, "granted": false})
	if l.Granted("ana", Art9) || !l.Granted("ana", ConductAttestation) || l.Granted("ben", Art9) {
		t.Fatal("current answers wrong")
	}
	if len(changes) != 3 {
		t.Fatalf("changes: %v", changes)
	}
	c, out := call(l, "GET", "ana", nil)
	if c != 200 || out["chain_ok"] != true || len(out["entries"].([]any)) != 3 {
		t.Fatalf("list: %d %v", c, out)
	}

	// A reload verifies; tampering with a stored entry is detected.
	l2, _ := New(ctx, st, members)
	if l2.Verify("ana") != nil || l2.Granted("ana", Art9) {
		t.Fatal("reload")
	}
	doc, _ := st.Get(ctx, "consent", "ana")
	var chain []Entry
	_ = json.Unmarshal(doc, &chain)
	chain[0].Granted = false
	doc, _ = json.Marshal(chain)
	_ = st.Put(ctx, "consent", "ana", doc)
	l3, _ := New(ctx, st, members)
	if l3.Verify("ana") == nil {
		t.Fatal("tampering not detected")
	}
	if _, broken := l3.VerifyAll(); broken != 1 {
		t.Fatal("VerifyAll missed it")
	}
	// A chain can't be moved to another member.
	_ = st.Put(ctx, "consent", "ben", doc)
	l4, _ := New(ctx, st, members)
	if l4.Verify("ben") == nil {
		t.Fatal("moved chain verified")
	}

	// Export and erasure (DSR).
	if v, _ := l.Export(ctx, "ana"); v == nil {
		t.Fatal("export empty")
	}
	rc, err := l.Erase(ctx, "ana")
	if err != nil || rc.Erased != 3 {
		t.Fatalf("erase: %+v %v", rc, err)
	}
	if v, _ := l.Export(ctx, "ana"); v != nil {
		t.Fatal("still there after erasure")
	}
}
