// Package safetysvc is the Trust & Safety service: member reports, the case queue, the
// moderator console API, enforcement and appeals (docs/spec/03_Backend.md §7.3, §8; ToS cl.
// 17–19; FR-033–FR-035, FR-116; OSA duties). It keeps its own database (SAFETY-DB).
//
//   - Members report a conversation or a profile, optionally disclosing messages they choose;
//     nothing else from an end-to-end encrypted thread ever reaches TRYST.
//   - On-device Guardian signals open cases automatically.
//   - Every case has a deadline from the escalation matrix (suspected minor 15 min; image
//     abuse, sextortion, threats 1 h; fraud and commercial sex 4 h; harassment 24 h).
//   - Moderators never see who reported; profile photos are blurred until they record why
//     they need to see them. Every console action is written to a hash-chained log.
//   - Decisions follow the enforcement ladder; a permanent removal needs a named decision-
//     maker and a second moderator's confirmation; appeals go to a different reviewer.
package safetysvc

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
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/enforcement"
	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/problem"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

// MemberAuth is satisfied by *identity.Server.
type MemberAuth interface {
	Authenticate(r *http.Request) (identity.Account, bool)
}

// Directory is what the safety service needs from the member platform (profilesvc).
type Directory interface {
	// Counterpart returns the other member of a thread the caller belongs to.
	Counterpart(me, threadID string) (string, bool)
	// Summary is the subject's pseudonym and city (no exact location).
	Summary(account string) (name, area string)
	// MainPhoto returns the subject's main photo bytes and content type.
	MainPhoto(ctx context.Context, account string) ([]byte, string, error)
	// SetStanding applies an enforcement outcome on the member platform.
	SetStanding(account, standing string)
}

// Category is what a member reports (02 ReportCategory), mapped to the escalation matrix
// (03 §7.3): human deadline, conduct for the enforcement ladder, and whether the subject is
// frozen automatically until a moderator decides.
type Category struct {
	Key         string
	Label       string
	Deadline    time.Duration
	Conduct     enforcement.Conduct
	AutoSuspend bool
}

var categories = []Category{
	{"minor", "Someone may be under 18", 15 * time.Minute, enforcement.Minor, true},
	{"ncii", "Sharing or threatening to share intimate images", time.Hour, enforcement.NCII, false},
	{"sextortion", "Blackmail with images or secrets", time.Hour, enforcement.Coercion, false},
	{"exposure_threat", "Threatening to tell my partner or others about me", time.Hour, enforcement.ExposedMember, false},
	{"coercion", "Pressure or control I didn't agree to", time.Hour, enforcement.Coercion, false},
	{"threat_violence", "Threats of violence", time.Hour, enforcement.Coercion, true},
	{"stalking", "Following me or turning up uninvited", time.Hour, enforcement.Coercion, true},
	{"romance_fraud", "Asking for money or a scam", 4 * time.Hour, enforcement.Harassment, false},
	{"commercial_sex", "Selling sex or paid services", 4 * time.Hour, enforcement.Harassment, false},
	{"impersonation", "Fake profile or not who they say", 24 * time.Hour, enforcement.Harassment, false},
	{"harassment", "Harassment or pressure after I said no", 24 * time.Hour, enforcement.Harassment, false},
	{"protected_trait_harassment", "Abuse about race, religion, disability, sexuality or gender", 24 * time.Hour, enforcement.Harassment, false},
	{"other", "Something else", 24 * time.Hour, enforcement.Harassment, false},
}

func category(key string) (Category, bool) {
	for _, c := range categories {
		if c.Key == key {
			return c, true
		}
	}
	return Category{}, false
}

// Case is one item in the moderation queue.
type Case struct {
	ID         string    `json:"id"`
	Subject    string    `json:"subject"`
	Reporter   string    `json:"reporter,omitempty"` // never shown to moderators
	Source     string    `json:"source"`             // report | guardian
	Category   string    `json:"category"`
	Thread     string    `json:"thread,omitempty"`
	Disclosed  []string  `json:"disclosed,omitempty"` // messages the reporter chose to share
	Note       string    `json:"note,omitempty"`
	Opened     time.Time `json:"opened"`
	Due        time.Time `json:"due"`
	State      string    `json:"state"` // open | pending_second | closed
	Decision   *Decision `json:"decision,omitempty"`
	Appeal     *Appeal   `json:"appeal,omitempty"`
	PhotoSeen  []string  `json:"photo_seen,omitempty"`  // staff who revealed the photo, with reasons
	Interim    bool      `json:"interim,omitempty"`     // subject frozen until decided (03 §7.3)
	MergedInto string    `json:"merged_into,omitempty"` // closed by the decision on another case
}

// Decision is an enforcement decision on a case.
type Decision struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"` // dismiss | warn | shadow_limit | suspend | permanent
	Ground    string    `json:"ground,omitempty"`
	DecidedBy string    `json:"decided_by"`
	ConfirmBy string    `json:"confirmed_by,omitempty"`
	At        time.Time `json:"at"`
	Reason    string    `json:"reason"`
	Anchors   bool      `json:"retain_anchors"`
}

// Appeal against a decision.
type Appeal struct {
	Filed    time.Time `json:"filed"`
	Due      time.Time `json:"due"`
	Text     string    `json:"text"`
	Reviewer string    `json:"reviewer,omitempty"`
	Outcome  string    `json:"outcome,omitempty"` // upheld | overturned
}

// Staff is an authenticated moderator.
type Staff struct {
	ID    string
	Roles []string
}

func (s Staff) has(role string) bool {
	for _, r := range s.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// StaffAuth authenticates the console. Production: the identity header set by the company
// sign-in proxy (Cloudflare Access / oauth2-proxy), believed only from TrustedProxies.
// Dev: "Authorization: Staff <token>" against DevTokens (refused in prod).
type StaffAuth struct {
	TrustedProxies []string
	DevTokens      map[string]Staff // sha256(token) hex -> staff
	Env            string
}

func (a StaffAuth) Authenticate(r *http.Request) (Staff, bool) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if identity.TrustedAddr(a.TrustedProxies, ip) {
		if id := r.Header.Get("X-Staff-Identity"); id != "" {
			return Staff{ID: id, Roles: strings.Split(r.Header.Get("X-Staff-Roles"), ",")}, true
		}
	}
	if a.Env == "dev" {
		if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Staff "); ok {
			h := sha256.Sum256([]byte(tok))
			if s, ok := a.DevTokens[hex.EncodeToString(h[:])]; ok {
				return s, true
			}
		}
	}
	return Staff{}, false
}

// Service is the HTTP handler for /v1/reports, /v1/appeals, /v1/me/standing and /ops/v1/*.
type Service struct {
	mu      sync.Mutex
	members MemberAuth
	staff   StaffAuth
	dir     Directory
	st      store.Store
	now     func() time.Time
	cases   map[string]*Case
	chain   []AuditEntry
	mux     *http.ServeMux
	// Notify records a masked notice for a member. May be nil.
	Notify func(account, kind, link string)
	// pending standing changes, applied after s.mu is released: the member platform calls
	// FromSignal while holding its own lock, so we never call into it while holding ours.
	pending [][2]string
}

var ErrDevTokensInProd = errors.New("staff dev tokens are forbidden outside the dev environment")

func New(members MemberAuth, staff StaffAuth, dir Directory, st store.Store) (*Service, error) {
	if staff.Env != "dev" && len(staff.DevTokens) > 0 {
		return nil, ErrDevTokensInProd
	}
	s := &Service{members: members, staff: staff, dir: dir, st: st, now: time.Now, cases: map[string]*Case{}, mux: http.NewServeMux()}
	if err := s.load(context.Background()); err != nil {
		return nil, err
	}
	m := s.mux
	m.HandleFunc("GET /v1/reports/categories", s.handleCategories)
	m.HandleFunc("POST /v1/reports", s.handleReport)
	m.HandleFunc("GET /v1/me/standing", s.handleStanding)
	m.HandleFunc("POST /v1/appeals", s.handleAppeal)
	m.HandleFunc("GET /ops/v1/cases", s.handleQueue)
	m.HandleFunc("GET /ops/v1/cases/{id}", s.handleCase)
	m.HandleFunc("GET /ops/v1/cases/{id}/photo", s.handleCasePhoto)
	m.HandleFunc("POST /ops/v1/cases/{id}/decision", s.handleDecision)
	m.HandleFunc("POST /ops/v1/cases/{id}/confirm", s.handleConfirm)
	m.HandleFunc("POST /ops/v1/cases/{id}/appeal", s.handleAppealDecision)
	m.HandleFunc("GET /ops/v1/audit/verify", s.handleAuditVerify)
	m.HandleFunc("GET /ops/v1/stats", s.handleStats)
	return s, nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	s.mux.ServeHTTP(w, r)
	s.applyPending()
}

func (s *Service) notify(account, kind string) {
	if s.Notify != nil {
		s.Notify(account, kind, "/account")
	}
}

func (s *Service) applyPending() {
	s.mu.Lock()
	pending := s.pending
	s.pending = nil
	s.mu.Unlock()
	for _, p := range pending {
		s.dir.SetStanding(p[0], p[1])
	}
}

// restand queues the subject's current standing for the member platform. Caller holds s.mu.
func (s *Service) restand(account string) {
	s.pending = append(s.pending, [2]string{account, s.standingOf(account)})
}

// closeRelated closes other open cases about the same conduct: one decision covers every
// report that corroborated it. Caller holds s.mu.
func (s *Service) closeRelated(ctx context.Context, c *Case, staff string) {
	for _, o := range s.cases {
		if o.ID != c.ID && o.Subject == c.Subject && o.Category == c.Category && o.State == "open" {
			o.State, o.MergedInto, o.Interim = "closed", c.ID, false
			s.save(ctx, o)
			s.audit(ctx, staff, "merge", o.ID, c.ID)
		}
	}
}

func fail(w http.ResponseWriter, status int, code, title string) {
	problem.Write(w, problem.Problem{Title: title, Status: status, Code: code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func newID(prefix string) string {
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// --- persistence ---

func (s *Service) load(ctx context.Context) error {
	docs, err := s.st.All(ctx, "case")
	if err != nil {
		return err
	}
	for _, b := range docs {
		var c Case
		if json.Unmarshal(b, &c) == nil {
			s.cases[c.ID] = &c
		}
	}
	chain, err := s.st.All(ctx, "audit")
	if err != nil {
		return err
	}
	for _, k := range store.Keys(chain) {
		var e AuditEntry
		if json.Unmarshal(chain[k], &e) == nil {
			s.chain = append(s.chain, e)
		}
	}
	sort.Slice(s.chain, func(i, j int) bool { return s.chain[i].Seq < s.chain[j].Seq })
	return VerifyChain(s.chain)
}

func (s *Service) save(ctx context.Context, c *Case) {
	b, _ := json.Marshal(c)
	if err := s.st.Put(ctx, "case", c.ID, b); err != nil {
		log.Printf("persist case failed: %v", err)
	}
}

// --- audit chain (ops.admin_action) ---

// AuditEntry is one console action. Hash covers the previous hash and this entry.
type AuditEntry struct {
	Seq    int       `json:"seq"`
	At     time.Time `json:"at"`
	Staff  string    `json:"staff"`
	Action string    `json:"action"`
	Case   string    `json:"case"`
	Detail string    `json:"detail"`
	Prev   string    `json:"prev"`
	Hash   string    `json:"hash"`
}

func (e AuditEntry) digest() string {
	h := sha256.New()
	fmt.Fprintf(h, "%d\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s", e.Seq, e.At.UTC().Format(time.RFC3339Nano), e.Staff, e.Action, e.Case, e.Detail, e.Prev)
	return hex.EncodeToString(h.Sum(nil))
}

var ErrTampered = errors.New("audit chain broken")

func VerifyChain(log []AuditEntry) error {
	prev := ""
	for i, e := range log {
		if e.Seq != i || e.Prev != prev || e.digest() != e.Hash {
			return fmt.Errorf("%w at %d", ErrTampered, i)
		}
		prev = e.Hash
	}
	return nil
}

func (s *Service) audit(ctx context.Context, staff, action, caseID, detail string) {
	prev := ""
	if n := len(s.chain); n > 0 {
		prev = s.chain[n-1].Hash
	}
	e := AuditEntry{Seq: len(s.chain), At: s.now(), Staff: staff, Action: action, Case: caseID, Detail: detail, Prev: prev}
	e.Hash = e.digest()
	s.chain = append(s.chain, e)
	b, _ := json.Marshal(e)
	if err := s.st.Put(ctx, "audit", fmt.Sprintf("%012d", e.Seq), b); err != nil {
		log.Printf("persist audit failed: %v", err)
	}
}

// --- intake ---

// FromSignal opens a case from an on-device Guardian signal (no message content).
func (s *Service) FromSignal(threadID, reporter, subject, kind string) {
	cat := map[string]string{"child_safety": "minor", "extortion": "sextortion", "fraud": "romance_fraud"}[kind]
	if cat == "" {
		return
	}
	c, _ := category(cat)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	cs := &Case{ID: newID("cs_"), Subject: subject, Reporter: reporter, Source: "guardian", Category: cat, Thread: threadID,
		Opened: now, Due: now.Add(c.Deadline), State: "open", Note: "On-device signal: " + kind, Interim: c.AutoSuspend}
	s.cases[cs.ID] = cs
	s.save(context.Background(), cs)
	if cs.Interim {
		s.restand(subject)
		// The member platform calls us holding its own lock; apply once it has returned.
		go s.applyPending()
	}
}

func (s *Service) handleCategories(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.members.Authenticate(r); !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	type c struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	}
	var out []c
	for _, x := range categories {
		out = append(out, c{x.Key, x.Label})
	}
	writeJSON(w, 200, map[string]any{"categories": out})
}

func (s *Service) handleReport(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.members.Authenticate(r)
	if !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	var in struct {
		Thread    string   `json:"thread_id"`
		Category  string   `json:"category"`
		Disclosed []string `json:"disclosed"`
		Note      string   `json:"note"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&in) != nil {
		fail(w, 422, "validation_failed", "Choose what happened.")
		return
	}
	cat, ok := category(in.Category)
	if !ok {
		fail(w, 422, "validation_failed", "Choose what happened.")
		return
	}
	subject, ok := s.dir.Counterpart(acc.ID, in.Thread)
	if !ok {
		fail(w, 404, "not_found", "Not found")
		return
	}
	if len(in.Disclosed) > 20 || len(in.Note) > 2000 {
		fail(w, 422, "validation_failed", "Share up to 20 messages and a short note.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	cs := &Case{ID: newID("cs_"), Subject: subject, Reporter: acc.ID, Source: "report", Category: cat.Key, Thread: in.Thread,
		Disclosed: in.Disclosed, Note: in.Note, Opened: now, Due: now.Add(cat.Deadline), State: "open", Interim: cat.AutoSuspend}
	s.cases[cs.ID] = cs
	s.save(r.Context(), cs)
	if cs.Interim {
		s.restand(subject) // frozen until a moderator decides (03 §7.3)
	}
	// Reporting never tells the other member.
	writeJSON(w, 201, map[string]any{"reference": cs.ID, "message": "Thank you. Our safety team will look at this. The other person won't be told you reported them."})
}

// --- member standing and appeals ---

func (s *Service) decisionsAbout(account string) []*Case {
	var out []*Case
	for _, c := range s.cases {
		if c.Subject == account && c.Decision != nil && c.State == "closed" && c.Decision.Action != "dismiss" {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Decision.At.Before(out[j].Decision.At) })
	return out
}

// standingOf is the member's current standing: the latest decision not overturned on appeal.
// Caller holds s.mu.
func (s *Service) standingOf(account string) string {
	standing := "good"
	for _, c := range s.decisionsAbout(account) {
		if c.Appeal == nil || c.Appeal.Outcome != "overturned" {
			standing = c.Decision.Action
		}
	}
	if standing != "permanent" && s.underReview(account) {
		standing = "suspend"
	}
	return standing
}

// underReview reports an interim freeze: an open case that freezes automatically, or a
// permanent removal waiting for the second moderator. Caller holds s.mu.
func (s *Service) underReview(account string) bool {
	for _, c := range s.cases {
		if c.Subject == account && ((c.Interim && c.State == "open") || c.State == "pending_second") {
			return true
		}
	}
	return false
}

func (s *Service) handleStanding(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.members.Authenticate(r)
	if !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	type item struct {
		Decision string    `json:"decision_id"`
		Action   string    `json:"action"`
		Reason   string    `json:"reason"`
		At       time.Time `json:"at"`
		Appeal   *Appeal   `json:"appeal,omitempty"`
	}
	items := []item{}
	for _, c := range s.decisionsAbout(acc.ID) {
		items = append(items, item{c.Decision.ID, c.Decision.Action, c.Decision.Reason, c.Decision.At, c.Appeal})
	}
	writeJSON(w, 200, map[string]any{"standing": s.standingOf(acc.ID), "under_review": s.underReview(acc.ID), "decisions": items})
}

func (s *Service) handleAppeal(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.members.Authenticate(r)
	if !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	var in struct {
		Decision string `json:"decision_id"`
		Text     string `json:"text"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&in) != nil || len(in.Text) < 10 || len(in.Text) > 4000 {
		fail(w, 422, "validation_failed", "Tell us in a few sentences why the decision is wrong.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.decisionsAbout(acc.ID) {
		if c.Decision.ID == in.Decision {
			if c.Appeal != nil {
				fail(w, 409, "already_appealed", "You've already appealed this decision.")
				return
			}
			now := s.now()
			c.Appeal = &Appeal{Filed: now, Due: enforcement.AppealDue(now), Text: in.Text}
			s.save(r.Context(), c)
			writeJSON(w, 201, map[string]any{"due": c.Appeal.Due, "message": "A different member of our team will review this within 10 working days."})
			return
		}
	}
	fail(w, 404, "not_found", "Not found")
}

// --- console ---

func (s *Service) staffOr401(w http.ResponseWriter, r *http.Request) (Staff, bool) {
	st, ok := s.staff.Authenticate(r)
	if !ok || !(st.has("moderator") || st.has("lead")) {
		fail(w, 401, "staff_required", "Staff sign-in required.")
		return Staff{}, false
	}
	return st, true
}

type caseSummary struct {
	ID       string    `json:"id"`
	Category string    `json:"category"`
	Label    string    `json:"label"`
	Source   string    `json:"source"`
	Opened   time.Time `json:"opened"`
	Due      time.Time `json:"due"`
	Overdue  bool      `json:"overdue"`
	State    string    `json:"state"`
	Appeal   bool      `json:"appeal_pending"`
}

func (s *Service) summary(c *Case) caseSummary {
	cat, _ := category(c.Category)
	appeal := c.Appeal != nil && c.Appeal.Outcome == ""
	due := c.Due
	if appeal {
		due = c.Appeal.Due
	}
	return caseSummary{c.ID, c.Category, cat.Label, c.Source, c.Opened, due, s.now().After(due) && (c.State != "closed" || appeal), c.State, appeal}
}

func (s *Service) handleQueue(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.staffOr401(w, r); !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []caseSummary
	for _, c := range s.cases {
		if c.State != "closed" || (c.Appeal != nil && c.Appeal.Outcome == "") {
			out = append(out, s.summary(c))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Due.Before(out[j].Due) })
	if out == nil {
		out = []caseSummary{}
	}
	writeJSON(w, 200, map[string]any{"cases": out})
}

func (s *Service) handleCase(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffOr401(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[r.PathValue("id")]
	if !ok {
		fail(w, 404, "not_found", "Not found")
		return
	}
	s.audit(r.Context(), st.ID, "view_case", c.ID, "")
	name, area := s.dir.Summary(c.Subject)
	prior := 0
	reporters := map[string]bool{}
	for _, o := range s.cases {
		if o.Subject != c.Subject {
			continue
		}
		if o.Reporter != "" && o.Source == "report" {
			reporters[o.Reporter] = true
		}
		if o.Decision != nil && o.State == "closed" && o.Decision.Action != "dismiss" && o.ID != c.ID {
			prior++
		}
	}
	writeJSON(w, 200, map[string]any{
		"case":                  s.summary(c),
		"subject":               map[string]any{"name": name, "area": area},
		"disclosed":             c.Disclosed,
		"note":                  c.Note,
		"reporter":              "hidden", // moderators never see who reported (03 §7.3)
		"independent_reporters": len(reporters),
		"prior_strikes":         prior,
		"decision":              c.Decision,
		"appeal":                c.Appeal,
	})
}

// handleCasePhoto reveals the subject's main photo only with a recorded reason.
func (s *Service) handleCasePhoto(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffOr401(w, r)
	if !ok {
		return
	}
	reason := strings.TrimSpace(r.URL.Query().Get("reason"))
	if len(reason) < 10 {
		fail(w, 422, "reason_required", "Record why you need to see the photo (at least 10 characters).")
		return
	}
	s.mu.Lock()
	c, ok := s.cases[r.PathValue("id")]
	if ok {
		c.PhotoSeen = append(c.PhotoSeen, st.ID+": "+reason)
		s.save(r.Context(), c)
		s.audit(r.Context(), st.ID, "reveal_photo", c.ID, reason)
	}
	s.mu.Unlock()
	if !ok {
		fail(w, 404, "not_found", "Not found")
		return
	}
	data, ctype, err := s.dir.MainPhoto(r.Context(), c.Subject)
	if err != nil {
		fail(w, 404, "not_found", "No photo")
		return
	}
	w.Header().Set("Content-Type", ctype)
	_, _ = w.Write(data)
}

var groundsByKey = map[string]enforcement.Ground{
	"": "", "g1": enforcement.G1SelfDeclared, "g2": enforcement.G2Corroborated, "g3": enforcement.G3CourtOrder,
	"g4": enforcement.G4LawEnforcement, "g5": enforcement.G5ModelConfirmed, "g6": enforcement.G6FalseAttest,
}

var actionNames = map[enforcement.Action]string{
	enforcement.Warn: "warn", enforcement.ShadowLimit: "shadow_limit", enforcement.Suspend: "suspend",
	enforcement.Permanent: "permanent", enforcement.LawEnforcementReferral: "referral",
}

// handleDecision applies the enforcement ladder. Moderators choose dismiss or uphold (with a
// ground where one applies); the action follows from the conduct, prior strikes and the
// number of independent reporters. A permanent removal waits for a second moderator.
func (s *Service) handleDecision(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffOr401(w, r)
	if !ok {
		return
	}
	var in struct {
		Outcome string `json:"outcome"` // dismiss | uphold
		Ground  string `json:"ground"`
		Reason  string `json:"reason"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&in) != nil || len(strings.TrimSpace(in.Reason)) < 10 {
		fail(w, 422, "validation_failed", "Give the reason the member will see (at least 10 characters).")
		return
	}
	g, ok := groundsByKey[in.Ground]
	if !ok {
		fail(w, 422, "validation_failed", "Unknown ground.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[r.PathValue("id")]
	if !ok || c.State != "open" {
		fail(w, 409, "not_open", "This case isn't open.")
		return
	}
	now := s.now()
	if in.Outcome == "dismiss" {
		c.Decision = &Decision{ID: newID("dc_"), Action: "dismiss", DecidedBy: st.ID, At: now, Reason: in.Reason}
		c.State, c.Interim = "closed", false
		s.save(r.Context(), c)
		s.audit(r.Context(), st.ID, "dismiss", c.ID, in.Reason)
		s.restand(c.Subject)
		writeJSON(w, 200, map[string]any{"decision": c.Decision})
		return
	}
	cat, _ := category(c.Category)
	prior, reporters := 0, map[string]bool{}
	for _, o := range s.cases {
		if o.Subject == c.Subject {
			if o.Source == "report" && o.Reporter != "" {
				reporters[o.Reporter] = true
			}
			if o.ID != c.ID && o.Decision != nil && o.State == "closed" && o.Decision.Action != "dismiss" {
				prior++
			}
		}
	}
	if c.Source == "guardian" && g == "" {
		g = enforcement.G5ModelConfirmed // a human has now confirmed the on-device signal
	}
	d, err := enforcement.Decide(cat.Conduct, g, prior, len(reporters), st.ID, now)
	if err != nil {
		fail(w, 422, "not_enough_grounds", "One uncorroborated report can't lead to permanent removal. Add a ground, or wait for corroboration.")
		return
	}
	c.Decision = &Decision{ID: newID("dc_"), Action: actionNames[d.Action], Ground: string(d.Ground), DecidedBy: st.ID, At: now, Reason: in.Reason, Anchors: d.RetainAnchors}
	if d.Action == enforcement.Permanent {
		// Two-person rule (03 §7.3). The member is suspended while the second moderator
		// looks: the conduct is serious enough that they shouldn't keep using TRYST meanwhile.
		c.State = "pending_second"
		s.audit(r.Context(), st.ID, "propose_permanent", c.ID, in.Reason)
	} else {
		c.State, c.Interim = "closed", false
		s.audit(r.Context(), st.ID, "decide_"+c.Decision.Action, c.ID, in.Reason)
		s.notify(c.Subject, "safety")
		s.closeRelated(r.Context(), c, st.ID)
	}
	s.save(r.Context(), c)
	s.restand(c.Subject)
	writeJSON(w, 200, map[string]any{"decision": c.Decision, "state": c.State})
}

func (s *Service) handleConfirm(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffOr401(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[r.PathValue("id")]
	if !ok || c.State != "pending_second" {
		fail(w, 409, "not_pending", "Nothing to confirm.")
		return
	}
	if c.Decision.DecidedBy == st.ID {
		fail(w, 403, "second_person_required", "A different moderator must confirm a permanent removal.")
		return
	}
	in := struct {
		Agree  *bool  `json:"agree"`
		Reason string `json:"reason"`
	}{}
	_ = json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&in)
	if in.Agree != nil && !*in.Agree {
		// The second moderator disagrees: the case goes back to the queue and the interim
		// suspension is lifted.
		c.Decision, c.State = nil, "open"
		s.save(r.Context(), c)
		s.restand(c.Subject)
		s.audit(r.Context(), st.ID, "reject_permanent", c.ID, in.Reason)
		writeJSON(w, 200, map[string]any{"state": c.State})
		return
	}
	c.Decision.ConfirmBy = st.ID
	c.State, c.Interim = "closed", false
	s.save(r.Context(), c)
	s.audit(r.Context(), st.ID, "confirm_permanent", c.ID, in.Reason)
	s.notify(c.Subject, "safety")
	s.closeRelated(r.Context(), c, st.ID)
	s.restand(c.Subject)
	writeJSON(w, 200, map[string]any{"decision": c.Decision})
}

func (s *Service) handleAppealDecision(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffOr401(w, r)
	if !ok {
		return
	}
	var in struct {
		MemberWins bool   `json:"member_wins"`
		Reason     string `json:"reason"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&in) != nil || len(in.Reason) < 10 {
		fail(w, 422, "validation_failed", "Give the reason the member will see.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cases[r.PathValue("id")]
	if !ok || c.Appeal == nil || c.Appeal.Outcome != "" {
		fail(w, 409, "no_appeal", "No open appeal on this case.")
		return
	}
	orig := enforcement.Decision{DecidedBy: c.Decision.DecidedBy, RetainAnchors: c.Decision.Anchors}
	out, err := enforcement.ResolveAppeal(orig, st.ID, in.MemberWins)
	if err != nil || (c.Decision.ConfirmBy != "" && c.Decision.ConfirmBy == st.ID) {
		fail(w, 403, "different_reviewer", "Appeals must be reviewed by someone who wasn't involved in the decision.")
		return
	}
	c.Appeal.Reviewer = st.ID
	if out.Upheld {
		c.Appeal.Outcome = "upheld"
	} else {
		c.Appeal.Outcome = "overturned"
	}
	s.save(r.Context(), c)
	s.audit(r.Context(), st.ID, "appeal_"+c.Appeal.Outcome, c.ID, in.Reason)
	s.notify(c.Subject, "appeal")
	s.restand(c.Subject)
	writeJSON(w, 200, map[string]any{"appeal": c.Appeal})
}

func (s *Service) handleAuditVerify(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.staffOr401(w, r); !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := VerifyChain(s.chain)
	writeJSON(w, 200, map[string]any{"ok": err == nil, "entries": len(s.chain)})
}

// QueueStats returns open cases, overdue cases, and overdue cases in the 15-minute and 1-hour
// categories, for the on-call alert rules.
func (s *Service) QueueStats() (open, overdue, overdueUrgent int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.cases {
		pending := c.State != "closed" || (c.Appeal != nil && c.Appeal.Outcome == "")
		if !pending {
			continue
		}
		open++
		sm := s.summary(c)
		if sm.Overdue {
			overdue++
			if cat, _ := category(c.Category); cat.Deadline <= time.Hour && c.State != "closed" {
				overdueUrgent++
			}
		}
	}
	return
}

// handleStats is the transparency-report input: counts only, no member data.
func (s *Service) handleStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.staffOr401(w, r); !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	byCat, byAction := map[string]int{}, map[string]int{}
	appeals, overturned, onTime, closed := 0, 0, 0, 0
	for _, c := range s.cases {
		byCat[c.Category]++
		if c.Decision != nil {
			byAction[c.Decision.Action]++
			closed++
			if !c.Decision.At.After(c.Due) {
				onTime++
			}
		}
		if c.Appeal != nil && c.Appeal.Outcome != "" {
			appeals++
			if c.Appeal.Outcome == "overturned" {
				overturned++
			}
		}
	}
	writeJSON(w, 200, map[string]any{"by_category": byCat, "by_action": byAction, "decided": closed, "decided_on_time": onTime,
		"appeals_decided": appeals, "overturned": overturned})
}
