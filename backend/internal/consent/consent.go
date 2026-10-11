// Package consent is the member's consent ledger (docs/spec/02_Shared_Contracts.md §4.2,
// §5.2; 03 §11; FR-040, FR-126; D-63): append-only and hash-chained, one chain per member.
// Every grant and withdrawal is a new entry; nothing is ever changed in place.
//
//	entry_hash = SHA-256(prev_hash ‖ canonical JSON of the record)
//
// The Article 9 consent (art9_processing) is separate from accepting the Terms and is
// needed for discovery; withdrawing it takes the member out of discovery at once.
package consent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/dsr"
	"github.com/jnnseya-cpu/tryst/backend/internal/problem"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

// Kinds (02 §2 consent_kind).
const (
	Art9               = "art9_processing"
	MediaView          = "media_view"
	Meet               = "meet"
	CoupleLink         = "couple_link"
	Retention          = "retention"
	ConductAttestation = "conduct_attestation"
	SafetyTrainingUse  = "safety_training_use"
	BiometricLogin     = "biometric_login"
)

var kinds = map[string]bool{Art9: true, MediaView: true, Meet: true, CoupleLink: true, Retention: true,
	ConductAttestation: true, SafetyTrainingUse: true, BiometricLogin: true}

// memberSet are the kinds a member grants or withdraws directly at /v1/consents. The others
// are recorded where they are given (the conduct declaration, a meet, a couple link).
var memberSet = map[string]bool{Art9: true, BiometricLogin: true, SafetyTrainingUse: true}

// Versions of the wording each directly-set consent was given against.
var Versions = map[string]string{Art9: "art9/2026-10", BiometricLogin: "biometric/2026-10", SafetyTrainingUse: "training/2026-10"}

// Entry is one ledger record.
type Entry struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"`
	Granted  bool      `json:"granted"`
	Scope    string    `json:"scope,omitempty"` // wording version, or what it applies to
	At       time.Time `json:"at"`
	PrevHash string    `json:"prev_hash"`
	Hash     string    `json:"hash"`
}

// hash covers the account, so a chain can't be moved to another member.
func hash(account, prev string, e Entry) string {
	rec, _ := json.Marshal(struct {
		Account string    `json:"account"`
		ID      string    `json:"id"`
		Kind    string    `json:"kind"`
		Granted bool      `json:"granted"`
		Scope   string    `json:"scope"`
		At      time.Time `json:"at"`
	}{account, e.ID, e.Kind, e.Granted, e.Scope, e.At.UTC()})
	p, _ := hex.DecodeString(prev)
	h := sha256.Sum256(append(p, rec...))
	return hex.EncodeToString(h[:])
}

// ErrBroken means a chain failed verification.
var ErrBroken = errors.New("consent chain broken")

// Ledger holds every member's chain.
type Ledger struct {
	st     store.Store
	now    func() time.Time
	mu     sync.Mutex
	chains map[string][]Entry
	// OnChange runs after an entry is appended (outside the ledger's lock), e.g. to take a
	// member out of discovery when they withdraw art9_processing.
	OnChange func(account, kind string, granted bool)
	// Members authenticates the caller for the HTTP API.
	Members func(r *http.Request) (string, bool)
	mux     *http.ServeMux
}

// New loads every chain and verifies it. A broken chain is reported, not repaired.
func New(ctx context.Context, st store.Store, members func(*http.Request) (string, bool)) (*Ledger, error) {
	l := &Ledger{st: st, now: time.Now, chains: map[string][]Entry{}, Members: members, mux: http.NewServeMux()}
	docs, err := st.All(ctx, "consent")
	if err != nil {
		return nil, err
	}
	for account, b := range docs {
		var chain []Entry
		if err := json.Unmarshal(b, &chain); err != nil {
			return nil, fmt.Errorf("consent chain: %w", err)
		}
		l.chains[account] = chain
		if verify(account, chain) != nil {
			log.Printf("ALERT consent chain failed verification (one member)") // no identifier logged
		}
	}
	l.mux.HandleFunc("GET /v1/consents", l.handleList)
	l.mux.HandleFunc("POST /v1/consents", l.handleAppend)
	return l, nil
}

func verify(account string, chain []Entry) error {
	prev := ""
	for _, e := range chain {
		if e.PrevHash != prev || hash(account, prev, e) != e.Hash {
			return ErrBroken
		}
		prev = e.Hash
	}
	return nil
}

// Verify checks one member's chain.
func (l *Ledger) Verify(account string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return verify(account, l.chains[account])
}

// VerifyAll checks every chain (nightly, 03 §11) and returns how many failed.
func (l *Ledger) VerifyAll() (checked, broken int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for a, c := range l.chains {
		checked++
		if verify(a, c) != nil {
			broken++
		}
	}
	return
}

// Append records a grant or withdrawal.
func (l *Ledger) Append(ctx context.Context, account, kind string, granted bool, scope string) (Entry, error) {
	if !kinds[kind] {
		return Entry{}, fmt.Errorf("unknown consent kind %q", kind)
	}
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	l.mu.Lock()
	chain := l.chains[account]
	prev := ""
	if n := len(chain); n > 0 {
		prev = chain[n-1].Hash
	}
	e := Entry{ID: "cn_" + hex.EncodeToString(b), Kind: kind, Granted: granted, Scope: scope, At: l.now().UTC(), PrevHash: prev}
	e.Hash = hash(account, prev, e)
	next := append(append([]Entry{}, chain...), e)
	doc, _ := json.Marshal(next)
	if err := l.st.Put(ctx, "consent", account, doc); err != nil {
		l.mu.Unlock()
		return Entry{}, err
	}
	l.chains[account] = next
	l.mu.Unlock()
	if l.OnChange != nil {
		l.OnChange(account, kind, granted)
	}
	return e, nil
}

// Granted reports the member's current answer for a kind (the latest entry).
func (l *Ledger) Granted(account, kind string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.chains[account]
	for i := len(c) - 1; i >= 0; i-- {
		if c[i].Kind == kind {
			return c[i].Granted
		}
	}
	return false
}

func (l *Ledger) current(account string) map[string]bool {
	out := map[string]bool{}
	for _, e := range l.chains[account] {
		out[e.Kind] = e.Granted
	}
	return out
}

func (l *Ledger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	l.mux.ServeHTTP(w, r)
}

func fail(w http.ResponseWriter, status int, code, title string) {
	problem.Write(w, problem.Problem{Title: title, Status: status, Code: code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (l *Ledger) handleList(w http.ResponseWriter, r *http.Request) {
	acc, ok := l.Members(r)
	if !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	l.mu.Lock()
	entries := append([]Entry{}, l.chains[acc]...)
	cur := l.current(acc)
	ok = verify(acc, entries) == nil
	l.mu.Unlock()
	writeJSON(w, 200, map[string]any{"entries": entries, "current": cur, "chain_ok": ok, "versions": Versions})
}

func (l *Ledger) handleAppend(w http.ResponseWriter, r *http.Request) {
	acc, ok := l.Members(r)
	if !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	var in struct {
		Kind    string `json:"kind"`
		Granted *bool  `json:"granted"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 4<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || in.Granted == nil || !memberSet[in.Kind] {
		fail(w, 422, "validation_failed", "Choose a consent you can give or withdraw here.")
		return
	}
	scope := ""
	if *in.Granted {
		scope = Versions[in.Kind]
	}
	e, err := l.Append(r.Context(), acc, in.Kind, *in.Granted, scope)
	if err != nil {
		log.Printf("consent append failed: %v", err)
		fail(w, 503, "unavailable", "We couldn't record that. Please try again.")
		return
	}
	writeJSON(w, 201, e)
}

// Name implements dsr.Part.
func (l *Ledger) Name() string { return "consents" }

// Export returns the member's whole chain (SAR, FR-039).
func (l *Ledger) Export(_ context.Context, account string) (any, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.chains[account]) == 0 {
		return nil, nil
	}
	return map[string]any{"entries": l.chains[account], "chain_ok": verify(account, l.chains[account]) == nil}, nil
}

// Erase removes the chain. Nothing is processed on these consents after erasure, so there
// is nothing left to demonstrate them for.
func (l *Ledger) Erase(ctx context.Context, account string) (dsr.Receipt, error) {
	l.mu.Lock()
	n := len(l.chains[account])
	l.mu.Unlock()
	if err := l.st.Delete(ctx, "consent", account); err != nil && !errors.Is(err, store.ErrNotFound) {
		return dsr.Receipt{}, err
	}
	l.mu.Lock()
	delete(l.chains, account)
	l.mu.Unlock()
	return dsr.Receipt{Part: "consents", Erased: n}, nil
}
