package dsr

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jnnseya-cpu/tryst/backend/internal/metrics"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/problem"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

// MemberAuth returns the signed-in member's account id (identity.Server, adapted in main;
// a function so this package needn't import identity, which implements Part).
type MemberAuth func(r *http.Request) (string, bool)

// ConfirmPhrase must be typed to delete an account.
const ConfirmPhrase = "delete my account"

// Proof is the signed completion proof (02 §7 step 5). It names the member only by a one-way
// code, so it can be kept and shown without being personal data.
type Proof struct {
	JobID       string    `json:"deletion_job_id"`
	Subject     string    `json:"subject"` // SHA-256 of the erased account id, domain-separated
	Started     time.Time `json:"started"`
	CompletedAt time.Time `json:"completed_at"`
	Receipts    []Receipt `json:"receipts"`
	KeyID       string    `json:"key_id"`
	Signature   string    `json:"signature"` // Ed25519 over the canonical JSON of the fields above
}

func (p Proof) signed() []byte {
	c := p
	c.Signature = ""
	b, _ := json.Marshal(c)
	return b
}

// Verify checks a proof against a public key.
func Verify(pub ed25519.PublicKey, p Proof) bool {
	sig, err := base64.StdEncoding.DecodeString(p.Signature)
	return err == nil && ed25519.Verify(pub, p.signed(), sig)
}

// SubjectCode is the one-way code a proof uses for an account.
func SubjectCode(account string) string {
	h := sha256.Sum256([]byte("tryst/erasure/v1\x00" + account))
	return hex.EncodeToString(h[:])
}

// job is an erasure in progress. The account id is kept only until it completes.
type job struct {
	ID       string    `json:"id"`
	Account  string    `json:"account,omitempty"`
	Started  time.Time `json:"started"`
	Done     []Receipt `json:"done"`
	Complete bool      `json:"complete"`
}

// Service handles /v1/account/export, DELETE /v1/account and /v1/deletion-proofs/*.
type Service struct {
	members MemberAuth
	parts   []Part // erasure order: key destruction and discovery removal first, identity last
	st      store.Store
	key     ed25519.PrivateKey
	keyID   string
	now     func() time.Time
	mu      sync.Mutex // one erasure runner at a time
	mux     *http.ServeMux
	// Ledger, if set, receives one record per completed erasure: the subject code and time,
	// no personal data. It is kept and backed up apart from the service databases so that
	// erasures can be applied again after a restore.
	Ledger store.Store
}

// LedgerEntry is one completed erasure.
type LedgerEntry struct {
	Subject     string    `json:"subject"`
	CompletedAt time.Time `json:"completed_at"`
}

// New wires the service. parts run in order on erasure.
func New(members MemberAuth, st store.Store, signingSeed []byte, parts ...Part) (*Service, error) {
	if len(signingSeed) != ed25519.SeedSize {
		return nil, errors.New("dsr: the proof signing key must be a 32-byte seed")
	}
	key := ed25519.NewKeyFromSeed(signingSeed)
	kid := sha256.Sum256(key.Public().(ed25519.PublicKey))
	s := &Service{members: members, parts: parts, st: st, key: key, keyID: hex.EncodeToString(kid[:8]), now: time.Now, mux: http.NewServeMux()}
	s.mux.HandleFunc("POST /v1/account/export", s.handleExport)
	s.mux.HandleFunc("DELETE /v1/account", s.handleDelete)
	s.mux.HandleFunc("GET /v1/deletion-proofs/key", s.handleKey)
	s.mux.HandleFunc("GET /v1/deletion-proofs/{id}", s.handleProof)
	return s, nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	s.mux.ServeHTTP(w, r)
}

// PublicKey returns the proof verification key.
func (s *Service) PublicKey() ed25519.PublicKey { return s.key.Public().(ed25519.PublicKey) }

func fail(w http.ResponseWriter, status int, code, title string) {
	problem.Write(w, problem.Problem{Title: title, Status: status, Code: code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Export collects every part's data for the signed-in member. It is built on request and
// streamed to them over their authenticated session; no copy is kept on the server.
func (s *Service) Export(ctx context.Context, account string) (map[string]any, error) {
	out := map[string]any{"generated_at": s.now().UTC(), "format": "tryst-member-export/v1"}
	for _, p := range s.parts {
		v, err := p.Export(ctx, account)
		if err != nil {
			return nil, err
		}
		if v != nil {
			out[p.Name()] = v
		}
	}
	return out, nil
}

func (s *Service) handleExport(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.members(r)
	if !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	out, err := s.Export(r.Context(), acc)
	if err != nil {
		log.Printf("export failed: %v", err)
		fail(w, 500, "export_failed", "We couldn't build your export. Please try again.")
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="member-data.json"`)
	writeJSON(w, 200, out)
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.members(r)
	if !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	var in struct {
		Confirm string `json:"confirm"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&in)
	if strings.ToLower(strings.TrimSpace(in.Confirm)) != ConfirmPhrase {
		fail(w, 422, "confirm_required", `Type "delete my account" to confirm.`)
		return
	}
	id, proof, err := s.EraseAccount(r.Context(), acc)
	switch {
	case id == "":
		fail(w, 503, "unavailable", "Please try again.")
	case err != nil:
		writeJSON(w, 202, map[string]any{"deletion_job_id": id, "status": "in_progress"})
	default:
		writeJSON(w, 200, map[string]any{"deletion_job_id": id, "status": "complete", "completion_proof": proof})
	}
}

// EraseAccount records an erasure job and runs it. If it can't finish now, it returns the
// job id with an error and the sweep completes it later. An empty id means nothing started.
func (s *Service) EraseAccount(ctx context.Context, account string) (string, *Proof, error) {
	j := &job{ID: "del_" + randHex(12), Account: account, Started: s.now().UTC()}
	if err := s.saveJob(ctx, j); err != nil {
		return "", nil, err
	}
	proof, err := s.run(ctx, j)
	if err != nil {
		// Each part is idempotent; the sweeper finishes the job.
		log.Printf("erasure %s incomplete, will retry: %v", j.ID, err)
		metrics.Default.Inc("tryst_erasure_incomplete_total")
		return j.ID, nil, err
	}
	return j.ID, proof, nil
}

// run erases every remaining part of a job and issues the proof.
func (s *Service) run(ctx context.Context, j *job) (*Proof, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	done := map[string]bool{}
	for _, rc := range j.Done {
		done[rc.Part] = true
	}
	for _, p := range s.parts {
		if done[p.Name()] {
			continue
		}
		rc, err := p.Erase(ctx, j.Account)
		if err != nil {
			_ = s.saveJob(ctx, j)
			return nil, err
		}
		rc.Part = p.Name()
		j.Done = append(j.Done, rc)
	}
	pr := Proof{JobID: j.ID, Subject: SubjectCode(j.Account), Started: j.Started, CompletedAt: s.now().UTC(), Receipts: j.Done, KeyID: s.keyID}
	pr.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(s.key, pr.signed()))
	if s.Ledger != nil {
		lb, _ := json.Marshal(LedgerEntry{Subject: pr.Subject, CompletedAt: pr.CompletedAt})
		if err := s.Ledger.Put(ctx, "erasure", j.ID, lb); err != nil {
			return nil, err // retried by the sweep; the ledger must not miss an erasure
		}
	}
	b, _ := json.Marshal(pr)
	if err := s.st.Put(ctx, "proof", j.ID, b); err != nil {
		return nil, err
	}
	j.Account, j.Complete = "", true // the job forgets who it was
	if err := s.st.Delete(ctx, "job", j.ID); err != nil {
		return nil, err
	}
	log.Printf("erasure %s complete in %s", j.ID, pr.CompletedAt.Sub(j.Started).Round(time.Millisecond))
	metrics.Default.Observe("tryst_erasure_seconds", pr.CompletedAt.Sub(j.Started).Seconds())
	return &pr, nil
}

// Sweep finishes erasures that were interrupted. Run it periodically.
func (s *Service) Sweep(ctx context.Context) {
	docs, err := s.st.All(ctx, "job")
	if err != nil {
		return
	}
	for _, b := range docs {
		var j job
		if json.Unmarshal(b, &j) == nil && !j.Complete && j.Account != "" {
			if _, err := s.run(ctx, &j); err != nil {
				log.Printf("erasure %s still incomplete: %v", j.ID, err)
			}
		}
	}
}

func (s *Service) saveJob(ctx context.Context, j *job) error {
	b, _ := json.Marshal(j)
	return s.st.Put(ctx, "job", j.ID, b)
}

func (s *Service) handleProof(w http.ResponseWriter, r *http.Request) {
	b, err := s.st.Get(r.Context(), "proof", r.PathValue("id"))
	if err != nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

func (s *Service) handleKey(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"alg": "Ed25519", "key_id": s.keyID, "public_key": base64.StdEncoding.EncodeToString(s.PublicKey())})
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
