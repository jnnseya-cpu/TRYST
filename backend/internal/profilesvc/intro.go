package profilesvc

// Matching and introductions (02 §5.3, §5.5, §5.6, §6.2, §6.3; FR-003, FR-013, FR-014,
// FR-079, FR-128; D-04, D-65).
//
//   - Intake: the member states who they are, who they want to meet, what they're looking
//     for, hard limits and interests, from controlled lists, and confirms it. Every
//     confirmed version is kept (FR-079).
//   - Slate: up to 18 cards a day. Stage 0 gates run first and are never traded off: both
//     members want each other's group, no hard limit is crossed, blocks, discretion, the
//     partner shield, hidden areas, tier floors, Article 9 consent, same city within both
//     distances. Then a weighted baseline score ranks the rest; about 15% of cards are
//     exploration. Every card carries 2-4 reason codes. Nothing on a slate is paid for.
//   - Intent: private. Three free a week; unlimited on TRYST+, DUO or ENVOY; otherwise 5
//     Keys (15 from a couple to a single member), refunded in full when they reply. The
//     recipient's inbound cap (12 a day) queues overflow; the sender always sees "sent".
//   - Handshake: when both want to meet (a mutual intent or an accepted one), each side
//     gets their own brief and decides. The conversation opens only when both accept; a
//     couple's side follows its veto mode.
//   - ENVOY proposes intents with reasons; nothing is sent until the member approves each.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	mrand "math/rand/v2"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/couple"
	"github.com/jnnseya-cpu/tryst/backend/internal/keys"
	"github.com/jnnseya-cpu/tryst/backend/internal/profile"
)

// Controlled vocabularies (contracts/vocab; 02 §6.3).
var (
	Identities = []string{"woman", "man", "nonbinary"}
	Groups     = []string{"women", "men", "nonbinary", "couples"}
	Shapes     = []string{"one_off", "recurring", "ongoing", "exploratory", "third", "couple"}
	Interests  = []string{"dinner", "drinks", "dancing", "art", "music", "outdoors", "travel", "wellness", "kink_curious", "slow_burn", "conversation", "spontaneous"}
)

const (
	slateSize        = 18
	explorationShare = 0.15
	defaultInboxCap  = keys.DefaultCap
	envoyProposals   = 3
)

// Vector is the member's confirmed IntentVector (the parts matching uses in P1).
type Vector struct {
	Version     int       `json:"version"`
	Identity    string    `json:"identity"`
	Seeking     []string  `json:"seeking"`
	Shapes      []string  `json:"intent_shape"`
	HardLimits  []string  `json:"hard_limits"`
	Interests   []string  `json:"interests"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

// Intent is one private expression of interest.
type Intent struct {
	ID          string    `json:"id"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	At          time.Time `json:"at"`
	Keys        int       `json:"keys"` // held, refunded on reply
	Free        bool      `json:"free"`
	Via         string    `json:"via"`   // slate | envoy
	State       string    `json:"state"` // queued | delivered | accepted | declined | mutual
	DeliveredAt time.Time `json:"delivered_at,omitempty"`
	Handshake   string    `json:"handshake,omitempty"`
}

// Handshake is a mutual introduction waiting for both sides to accept.
type Handshake struct {
	ID        string            `json:"id"`
	Sides     [2][]string       `json:"sides"` // each side: the member, plus their partner if a couple
	Leads     [2]string         `json:"leads"` // the two members whose intents met
	Created   time.Time         `json:"created"`
	Decisions map[string]string `json:"decisions"` // account -> accept | decline
	State     string            `json:"state"`     // open | accepted | declined
	ThreadID  string            `json:"thread_id,omitempty"`
}

type introState struct {
	intents    map[string]*Intent
	handshakes map[string]*Handshake
	passed     map[string]map[string]bool
	saved      map[string]map[string]bool
	couples    map[string]*coupleLink
	invites    map[string]*coupleInvite // token hash -> invite
}

func newIntroState() *introState {
	return &introState{intents: map[string]*Intent{}, handshakes: map[string]*Handshake{}, passed: map[string]map[string]bool{},
		saved: map[string]map[string]bool{}, couples: map[string]*coupleLink{}, invites: map[string]*coupleInvite{}}
}

func introID(prefix string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func (s *Service) loadIntro(ctx context.Context) error {
	load := func(kind string, f func([]byte) error) error {
		docs, err := s.st.All(ctx, kind)
		if err != nil {
			return err
		}
		for id, b := range docs {
			if err := f(b); err != nil {
				return fmt.Errorf("%s %s: %w", kind, id, err)
			}
		}
		return nil
	}
	if err := load("intent", func(b []byte) error {
		var i Intent
		err := json.Unmarshal(b, &i)
		s.in.intents[i.ID] = &i
		return err
	}); err != nil {
		return err
	}
	if err := load("handshake", func(b []byte) error {
		var h Handshake
		err := json.Unmarshal(b, &h)
		s.in.handshakes[h.ID] = &h
		return err
	}); err != nil {
		return err
	}
	if err := load("couple", func(b []byte) error {
		var c coupleLink
		err := json.Unmarshal(b, &c)
		s.in.couples[c.ID] = &c
		return err
	}); err != nil {
		return err
	}
	return load("slate_marks", func(b []byte) error {
		var m struct {
			Account string   `json:"account"`
			Passed  []string `json:"passed"`
			Saved   []string `json:"saved"`
		}
		if err := json.Unmarshal(b, &m); err != nil {
			return err
		}
		s.in.passed[m.Account], s.in.saved[m.Account] = setOf(m.Passed), setOf(m.Saved)
		return nil
	})
}

func setOf(xs []string) map[string]bool {
	out := map[string]bool{}
	for _, x := range xs {
		out[x] = true
	}
	return out
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *Service) put(ctx context.Context, kind, id string, v any) {
	b, _ := json.Marshal(v)
	if err := s.st.Put(ctx, kind, id, b); err != nil {
		log.Printf("persist %s failed: %v", kind, err)
	}
}

func (s *Service) saveMarks(ctx context.Context, account string) {
	s.put(ctx, "slate_marks", account, map[string]any{"account": account, "passed": keysOf(s.in.passed[account]), "saved": keysOf(s.in.saved[account])})
}

// ---------------------------------------------------------------- intake

func validVector(v Vector) bool {
	if !slices.Contains(Identities, v.Identity) || len(v.Seeking) == 0 || len(v.Shapes) == 0 || len(v.Interests) > 6 {
		return false
	}
	for _, list := range []struct {
		vals  []string
		vocab []string
	}{{v.Seeking, Groups}, {v.Shapes, Shapes}, {v.HardLimits, Shapes}, {v.Interests, Interests}} {
		for _, x := range list.vals {
			if !slices.Contains(list.vocab, x) {
				return false
			}
		}
	}
	for _, h := range v.HardLimits {
		if slices.Contains(v.Shapes, h) {
			return false // can't both want and refuse the same thing
		}
	}
	return true
}

func (s *Service) handleVector(w http.ResponseWriter, r *http.Request) {
	_, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	writeJSON(w, 200, map[string]any{"vector": p.Vector, "history": len(p.VectorHistory),
		"vocab": map[string]any{"identity": Identities, "seeking": Groups, "intent_shape": Shapes, "hard_limits": Shapes, "interests": Interests}})
}

func (s *Service) handleVectorConfirm(w http.ResponseWriter, r *http.Request) {
	_, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var v Vector
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSON))
	dec.DisallowUnknownFields()
	if dec.Decode(&v) != nil || !validVector(v) {
		fail(w, 422, "validation_failed", "Choose from the lists: who you are, who you want to meet and what you're looking for.")
		return
	}
	v.Version, v.ConfirmedAt = len(p.VectorHistory)+1, s.now().UTC()
	if v.HardLimits == nil {
		v.HardLimits = []string{}
	}
	p.Vector = &v
	p.VectorHistory = append(p.VectorHistory, v)
	writeJSON(w, 200, map[string]any{"vector": v})
}

// ---------------------------------------------------------------- Stage 0 and scoring

// group is how others see a member: their identity, or "couples" while linked.
func (s *Service) group(account string) string {
	if s.coupleOf(account) != nil {
		return "couples"
	}
	p := s.data[account]
	if p == nil || p.Vector == nil {
		return ""
	}
	return map[string]string{"woman": "women", "man": "men", "nonbinary": "nonbinary"}[p.Vector.Identity]
}

func crosses(limits, shapes []string) bool {
	for _, l := range limits {
		if slices.Contains(shapes, l) {
			return true
		}
	}
	return false
}

// stage0 runs every hard gate for a pair. Caller holds the lock.
func (s *Service) stage0(viewer, member string) bool {
	v, m := s.data[viewer], s.data[member]
	if v == nil || m == nil || v.Vector == nil || m.Vector == nil || !s.ready(m) || m.Tier < s.MinDiscoveryTier ||
		!s.art9(member) || !s.eligible(viewer, member) || s.samePair(viewer, member) {
		return false
	}
	for _, a := range []string{viewer, member} {
		if c := s.coupleOf(a); c != nil && c.lapsed(s.now()) {
			return false // a couple that hasn't reconfirmed leaves discovery (FR-096)
		}
	}
	if !slices.Contains(v.Vector.Seeking, s.group(member)) || !slices.Contains(m.Vector.Seeking, s.group(viewer)) {
		return false
	}
	if crosses(v.Vector.HardLimits, m.Vector.Shapes) || crosses(m.Vector.HardLimits, v.Vector.Shapes) {
		return false
	}
	va, ok1 := areaByID(v.AreaID)
	ma, ok2 := areaByID(m.AreaID)
	if !ok1 || !ok2 || va.City != ma.City {
		return false
	}
	km, err := profile.CellDistanceKm(va.Cell, ma.Cell)
	return err == nil && km <= float64(min(v.policy().MaxDistanceKm, m.policy().MaxDistanceKm))
}

// samePair: partners in a couple are never introduced to each other.
func (s *Service) samePair(a, b string) bool {
	c := s.coupleOf(a)
	return c != nil && c.partner(a) == b
}

func overlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := 0
	for _, x := range a {
		if slices.Contains(b, x) {
			n++
		}
	}
	return float64(n) / float64(len(a)+len(b)-n)
}

type scored struct {
	id      string
	score   float64
	km      float64
	reasons []string
	explore bool
}

// score is the weighted baseline (03 §5): intent 0.4, interests 0.3, distance 0.2,
// verification parity 0.1. Reasons name only what both share.
func (s *Service) score(viewer, member string) scored {
	v, m := s.data[viewer], s.data[member]
	va, _ := areaByID(v.AreaID)
	ma, _ := areaByID(m.AreaID)
	km, _ := profile.CellDistanceKm(va.Cell, ma.Cell)
	io, ii := overlap(v.Vector.Shapes, m.Vector.Shapes), overlap(v.Vector.Interests, m.Vector.Interests)
	near := 1 - math.Min(km, 50)/50
	parity := 0.0
	if v.Tier == m.Tier {
		parity = 1
	}
	out := scored{id: member, km: km, score: 0.4*io + 0.3*ii + 0.2*near + 0.1*parity}
	if io > 0 {
		out.reasons = append(out.reasons, "intent_match")
	}
	if ii > 0 {
		out.reasons = append(out.reasons, "shared_interests")
	}
	if km <= 10 {
		out.reasons = append(out.reasons, "nearby")
	} else {
		out.reasons = append(out.reasons, "same_city")
	}
	out.reasons = append(out.reasons, "both_verified")
	if len(out.reasons) > 4 {
		out.reasons = out.reasons[:4]
	}
	return out
}

// slateFor ranks today's candidates. Caller holds the lock.
func (s *Service) slateFor(viewer string) []scored {
	var all []scored
	for id := range s.data {
		if id == viewer || s.in.passed[viewer][id] || !s.stage0(viewer, id) {
			continue
		}
		all = append(all, s.score(viewer, id))
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].score != all[j].score {
			return all[i].score > all[j].score
		}
		return all[i].id < all[j].id
	})
	if len(all) <= slateSize {
		return all
	}
	explore := int(math.Round(slateSize * explorationShare))
	top := append([]scored{}, all[:slateSize-explore]...)
	rest := all[slateSize-explore:]
	// Exploration: a seeded daily draw from below the cut, so a slate is stable for the day.
	day := s.now().UTC().Format("2006-01-02")
	h := sha256.Sum256([]byte(viewer + "|" + day))
	rng := mrand.New(mrand.NewPCG(binary.BigEndian.Uint64(h[:8]), 7)) // #nosec G404 -- a deterministic daily shuffle, not a secret
	rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	for _, c := range rest[:explore] {
		c.explore = true
		top = append(top, c)
	}
	return top
}

type card struct {
	Handle      string   `json:"handle"`
	DisplayName string   `json:"display_name"`
	Age         *int     `json:"age"`
	Area        string   `json:"area"`
	Distance    string   `json:"distance"`
	Couple      bool     `json:"couple"`
	Shapes      []string `json:"shared_intent"`
	Interests   []string `json:"shared_interests"`
	Photos      int      `json:"photos"`
	Reasons     []string `json:"explanation_codes"`
	PaidLabel   bool     `json:"paid_label"`
	Exploration bool     `json:"exploration"`
	Saved       bool     `json:"saved"`
	IntentSent  bool     `json:"intent_sent"`
}

func shared(a, b []string) []string {
	out := []string{}
	for _, x := range a {
		if slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

func (s *Service) cardFor(viewer string, c scored) card {
	v, m := s.data[viewer], s.data[c.id]
	va, _ := areaByID(v.AreaID)
	ma, _ := areaByID(m.AreaID)
	label, _ := profile.DistanceBand(va.Cell, ma.Cell)
	shown, _ := s.visiblePhotos(m)
	return card{Handle: s.tn.handle(viewer, c.id), DisplayName: m.DisplayName, Age: s.ageOf(m), Area: ma.Label(m.CityOnly),
		Distance: label, Couple: s.coupleOf(c.id) != nil, Shapes: shared(v.Vector.Shapes, m.Vector.Shapes),
		Interests: shared(v.Vector.Interests, m.Vector.Interests), Photos: len(shown), Reasons: c.reasons,
		Exploration: c.explore, Saved: s.in.saved[viewer][c.id], IntentSent: s.sentTo(viewer, c.id) != nil}
}

// introReady writes why the member can't use introductions yet. Caller holds the lock.
func (s *Service) introReady(w http.ResponseWriter, account string, p *memberProfile) bool {
	switch {
	case p.Tier < s.MinDiscoveryTier:
		fail(w, 403, "tier_required", "Finish your ID check to see introductions.")
	case hidden(p):
		fail(w, 409, "paused", "You're out of discovery. Come back from your account first.")
	case !s.art9(account):
		fail(w, 403, "consent_required", "Introductions need your consent to process what your profile says about your sex life.")
	case !s.ready(p):
		fail(w, 409, "profile_incomplete", "Finish your profile first: a main photo, a name and your area.")
	case p.Vector == nil:
		fail(w, 409, "intake_required", "Tell us what you're looking for first.")
	default:
		return true
	}
	return false
}

func (s *Service) handleSlate(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	if !s.introReady(w, acc.ID, p) {
		return
	}
	cards := []card{}
	for _, c := range s.slateFor(acc.ID) {
		cards = append(cards, s.cardFor(acc.ID, c))
	}
	writeJSON(w, 200, map[string]any{"cards": cards, "free_left": s.freeLeft(acc.ID), "unlimited": s.unlimited(acc.ID)})
}

// introResolve maps a viewer's handle to a member who still passes Stage 0.
func (s *Service) introResolve(viewer, handle string) (string, bool) {
	id, ok := s.tn.handles[viewer][handle]
	if !ok || !s.stage0(viewer, id) {
		return "", false
	}
	return id, true
}

func (s *Service) handleSlatePhoto(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	id, ok := s.introResolve(acc.ID, r.PathValue("handle"))
	n, err := strconv.Atoi(r.PathValue("n"))
	if !ok || err != nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	shown, _ := s.visiblePhotos(s.data[id])
	if n < 0 || n >= len(shown) {
		fail(w, 404, "not_found", "Not found")
		return
	}
	data, err := s.photoBytes(r.Context(), id, shown[n])
	if err != nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	w.Header().Set("Content-Type", shown[n].ContentType)
	_, _ = w.Write(data)
}

func (s *Service) handleMark(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, _, ok := s.member(w, r)
		if !ok {
			return
		}
		defer s.mu.Unlock()
		var in struct {
			Handle string `json:"handle"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in)
		id, ok := s.tn.handles[acc.ID][in.Handle]
		if !ok {
			fail(w, 404, "not_found", "Not found")
			return
		}
		m := s.in.passed
		if kind == "save" {
			m = s.in.saved
		}
		if m[acc.ID] == nil {
			m[acc.ID] = map[string]bool{}
		}
		m[acc.ID][id] = true
		s.saveMarks(r.Context(), acc.ID)
		w.WriteHeader(204)
	}
}

// ---------------------------------------------------------------- intents

func (s *Service) sentTo(from, to string) *Intent {
	for _, i := range s.in.intents {
		if i.From == from && i.To == to {
			return i
		}
	}
	return nil
}

func (s *Service) unlimited(account string) bool {
	if s.Plan == nil {
		return false
	}
	switch s.Plan(account) {
	case "tryst_plus", "tryst_plus_annual", "duo", "envoy":
		return true
	}
	return false
}

func (s *Service) freeLeft(account string) int {
	cut := s.now().Add(-7 * 24 * time.Hour)
	used := 0
	for _, i := range s.in.intents {
		if i.From == account && i.Free && i.At.After(cut) {
			used++
		}
	}
	return max(0, keys.FreeIntentsWeek-used)
}

func (s *Service) inboxCount(to string) int {
	cut := s.now().Add(-24 * time.Hour)
	n := 0
	for _, i := range s.in.intents {
		if i.To == to && !i.DeliveredAt.IsZero() && i.DeliveredAt.After(cut) {
			n++
		}
	}
	return n
}

// deliverQueued releases queued intents for a recipient as their cap allows (oldest first).
func (s *Service) deliverQueued(ctx context.Context, to string) {
	var q []*Intent
	for _, i := range s.in.intents {
		if i.To == to && i.State == "queued" {
			q = append(q, i)
		}
	}
	sort.Slice(q, func(a, b int) bool { return q[a].At.Before(q[b].At) })
	for _, i := range q {
		if s.inboxCount(to) >= defaultInboxCap || !s.stage0(to, i.From) {
			continue
		}
		i.State, i.DeliveredAt = "delivered", s.now()
		s.put(ctx, "intent", i.ID, i)
		s.notify(to, "intent", "/introductions")
	}
}

var errNoKeys = errors.New("keys")

// sendIntent creates an intent, or a handshake if the other side already wants to meet.
// Caller holds the lock; Keys are held through commerce.
func (s *Service) sendIntent(ctx context.Context, from, to, via string) (*Intent, error) {
	if prev := s.sentTo(from, to); prev != nil {
		return prev, nil
	}
	in := &Intent{ID: introID("in_"), From: from, To: to, At: s.now(), Via: via}
	switch {
	case s.unlimited(from):
	case s.freeLeft(from) > 0:
		in.Free = true
	default:
		toSolo := s.coupleOf(to) == nil
		in.Keys = keys.Cost(s.coupleOf(from) != nil, toSolo)
		if s.SpendKeys == nil {
			return nil, errNoKeys
		}
		if err := s.SpendKeys(ctx, from, in.ID, in.Keys); err != nil {
			return nil, errNoKeys
		}
	}
	// Their intent to me already exists: this is mutual.
	if back := s.sentTo(to, from); back != nil && (back.State == "delivered" || back.State == "queued") {
		in.State = "mutual"
		back.State = "mutual"
		h := s.openHandshake(ctx, to, from)
		in.Handshake, back.Handshake = h.ID, h.ID
		s.refund(ctx, back)
		s.put(ctx, "intent", back.ID, back)
		defer s.refund(ctx, in) // answered at once: refunded as on any reply
	} else if s.inboxCount(to) < defaultInboxCap {
		in.State, in.DeliveredAt = "delivered", s.now()
		s.notify(to, "intent", "/introductions")
	} else {
		in.State = "queued" // the sender is never told
	}
	s.in.intents[in.ID] = in
	s.put(ctx, "intent", in.ID, in)
	return in, nil
}

func (s *Service) refund(ctx context.Context, i *Intent) {
	if i.Keys > 0 && s.RefundKeys != nil {
		s.RefundKeys(ctx, i.From, i.ID)
	}
}

// senderState is what the sender sees: queued looks like sent.
func senderState(i *Intent) string {
	switch i.State {
	case "queued", "delivered", "declined":
		return "sent" // a decline is never shown to the sender
	}
	return i.State
}

func (s *Service) handleIntent(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	if !s.introReady(w, acc.ID, p) {
		return
	}
	var in struct {
		Target string `json:"target"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in)
	to, ok := s.introResolve(acc.ID, in.Target)
	if !ok {
		fail(w, 404, "not_found", "This member is no longer available.")
		return
	}
	i, err := s.sendIntent(r.Context(), acc.ID, to, "slate")
	if err != nil {
		fail(w, 402, "keys_insufficient", "You've used this week's free intents. TRYST+ gives unlimited intents.")
		return
	}
	writeJSON(w, 202, map[string]any{"intent_id": i.ID, "state": senderState(i), "handshake": i.Handshake, "free_left": s.freeLeft(acc.ID)})
}

type inbound struct {
	ID string `json:"id"`
	card
}

func (s *Service) handleIntents(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	s.deliverQueued(r.Context(), acc.ID)
	in := []inbound{}
	if p.Vector != nil {
		for _, i := range s.in.intents {
			if i.To == acc.ID && i.State == "delivered" && s.stage0(acc.ID, i.From) {
				c := s.score(acc.ID, i.From)
				in = append(in, inbound{i.ID, s.cardFor(acc.ID, c)})
			}
		}
	}
	sort.Slice(in, func(a, b int) bool { return in[a].ID < in[b].ID })
	out := []map[string]any{}
	for _, i := range s.in.intents {
		if i.From == acc.ID {
			name := ""
			if o := s.data[i.To]; o != nil {
				name = o.DisplayName
			}
			out = append(out, map[string]any{"id": i.ID, "to": name, "state": senderState(i), "at": i.At, "keys": i.Keys, "via": i.Via})
		}
	}
	hs := []map[string]any{}
	for _, h := range s.in.handshakes {
		if side := h.side(acc.ID); side >= 0 {
			hs = append(hs, s.handshakeView(acc.ID, h))
		}
	}
	sort.Slice(hs, func(a, b int) bool { return hs[a]["id"].(string) < hs[b]["id"].(string) })
	writeJSON(w, 200, map[string]any{"inbound": in, "outbound": out, "handshakes": hs, "free_left": s.freeLeft(acc.ID), "unlimited": s.unlimited(acc.ID)})
}

// handleReply lets the recipient answer an intent. Either answer refunds the sender's Keys.
func (s *Service) handleReply(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		Decision string `json:"decision"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in)
	i := s.in.intents[r.PathValue("id")]
	if i == nil || i.To != acc.ID || i.State != "delivered" || (in.Decision != "accept" && in.Decision != "decline") {
		fail(w, 404, "not_found", "Not found")
		return
	}
	s.refund(r.Context(), i)
	if in.Decision == "decline" {
		i.State = "declined"
		s.put(r.Context(), "intent", i.ID, i)
		w.WriteHeader(204)
		return
	}
	if !s.stage0(acc.ID, i.From) {
		fail(w, 404, "not_found", "This member is no longer available.")
		return
	}
	h := s.openHandshake(r.Context(), i.From, acc.ID)
	i.State, i.Handshake = "mutual", h.ID
	s.put(r.Context(), "intent", i.ID, i)
	writeJSON(w, 200, s.handshakeView(acc.ID, h))
}

// ---------------------------------------------------------------- handshake

func (h *Handshake) side(account string) int {
	for n, side := range h.Sides {
		if slices.Contains(side, account) {
			return n
		}
	}
	return -1
}

func (s *Service) sideOf(account string) []string {
	if c := s.coupleOf(account); c != nil {
		return []string{account, c.partner(account)}
	}
	return []string{account}
}

func (s *Service) openHandshake(ctx context.Context, a, b string) *Handshake {
	for _, h := range s.in.handshakes {
		if (h.Leads == [2]string{a, b} || h.Leads == [2]string{b, a}) && h.State == "open" {
			return h
		}
	}
	h := &Handshake{ID: introID("hs_"), Sides: [2][]string{s.sideOf(a), s.sideOf(b)}, Leads: [2]string{a, b}, Created: s.now(), Decisions: map[string]string{}, State: "open"}
	s.in.handshakes[h.ID] = h
	s.put(ctx, "handshake", h.ID, h)
	for _, side := range h.Sides {
		for _, m := range side {
			s.notify(m, "intro", "/introductions")
		}
	}
	return h
}

func toDecision(d string) couple.Decision {
	switch d {
	case "accept":
		return couple.Accept
	case "decline":
		return couple.Decline
	}
	return couple.Pending
}

// sideDecision applies a couple's veto mode to its side. Caller holds the lock.
func (s *Service) sideDecision(h *Handshake, n int) couple.Decision {
	side := h.Sides[n]
	if len(side) == 1 {
		return toDecision(h.Decisions[side[0]])
	}
	mode := "either"
	if c := s.coupleOf(side[0]); c != nil {
		mode = c.VetoMode
	}
	return couple.CoupleDecision(mode, toDecision(h.Decisions[side[0]]), toDecision(h.Decisions[side[1]]))
}

// handshakeView is the caller's own, asymmetric brief (02 §6.2): the other side's card and
// what the two sides share, never the other side's full vector or their decision.
func (s *Service) handshakeView(viewer string, h *Handshake) map[string]any {
	n := h.side(viewer)
	other := h.Leads[1-n]
	out := map[string]any{"id": h.ID, "state": h.State, "your_decision": h.Decisions[viewer]}
	if o := s.data[other]; o != nil && s.data[viewer] != nil && s.data[viewer].Vector != nil && o.Vector != nil {
		c := s.score(viewer, other)
		cd := s.cardFor(viewer, c)
		out["brief"] = map[string]any{"display_name": cd.DisplayName, "age": cd.Age, "area": cd.Area, "distance": cd.Distance,
			"couple": cd.Couple, "shared_intent": cd.Shapes, "shared_interests": cd.Interests, "explanation_codes": cd.Reasons,
			"handle": cd.Handle, "photos": cd.Photos}
	}
	if len(h.Sides[n]) == 2 {
		out["partner_decision"] = h.Decisions[h.Sides[n][1-slices.Index(h.Sides[n], viewer)]] // partners see each other's choice
	}
	if h.State == "accepted" {
		out["thread_id"] = h.ThreadID
	}
	return out
}

func (s *Service) handleBrief(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	h := s.in.handshakes[r.PathValue("id")]
	if h == nil || h.side(acc.ID) < 0 {
		fail(w, 404, "not_found", "Not found")
		return
	}
	writeJSON(w, 200, s.handshakeView(acc.ID, h))
}

func (s *Service) handleDecision(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		Decision string `json:"decision"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in)
	h := s.in.handshakes[r.PathValue("id")]
	if h == nil || h.side(acc.ID) < 0 {
		fail(w, 404, "not_found", "Not found")
		return
	}
	if (in.Decision != "accept" && in.Decision != "decline") || h.State != "open" {
		fail(w, 409, "handshake_closed", "This introduction is no longer open.")
		return
	}
	h.Decisions[acc.ID] = in.Decision
	a, b := s.sideDecision(h, 0), s.sideDecision(h, 1)
	switch {
	case a == couple.Decline || b == couple.Decline:
		h.State = "declined" // nobody is told who declined
	case a == couple.Accept && b == couple.Accept:
		if !s.stage0(h.Leads[0], h.Leads[1]) {
			h.State = "declined"
			break
		}
		h.State = "accepted"
		h.ThreadID = s.ensureThread(r.Context(), h.Leads[0], h.Leads[1], "handshake").ID
	}
	s.put(r.Context(), "handshake", h.ID, h)
	writeJSON(w, 200, s.handshakeView(acc.ID, h))
}

// ---------------------------------------------------------------- ENVOY

func (s *Service) envoy(account string) bool { return s.Plan != nil && s.Plan(account) == "envoy" }

var reasonText = map[string]string{
	"intent_match":     "You want the same kind of thing",
	"shared_interests": "You share interests",
	"nearby":           "Close by",
	"same_city":        "In your city",
	"both_verified":    "Both verified to the same level",
}

func (s *Service) handleEnvoyProposals(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	if !s.envoy(acc.ID) {
		fail(w, 403, "entitlement_required", "ENVOY isn't part of your plan.")
		return
	}
	if !s.introReady(w, acc.ID, p) {
		return
	}
	props := []map[string]any{}
	for _, c := range s.slateFor(acc.ID) {
		if len(props) == envoyProposals {
			break
		}
		if s.sentTo(acc.ID, c.id) != nil || c.explore {
			continue
		}
		var why []string
		for _, r := range c.reasons {
			why = append(why, reasonText[r])
		}
		props = append(props, map[string]any{"card": s.cardFor(acc.ID, c), "rationale": why})
	}
	writeJSON(w, 200, map[string]any{"proposals": props, "note": "Nothing is sent until you approve it."})
}

// handleEnvoyApprove sends one proposed intent. Every outbound intent needs this tap (D-04).
func (s *Service) handleEnvoyApprove(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	if !s.envoy(acc.ID) {
		fail(w, 403, "entitlement_required", "ENVOY isn't part of your plan.")
		return
	}
	if !s.introReady(w, acc.ID, p) {
		return
	}
	to, ok := s.introResolve(acc.ID, r.PathValue("handle"))
	if !ok {
		fail(w, 404, "not_found", "This member is no longer available.")
		return
	}
	i, err := s.sendIntent(r.Context(), acc.ID, to, "envoy")
	if err != nil {
		fail(w, 402, "keys_insufficient", "Not enough Keys.")
		return
	}
	writeJSON(w, 202, map[string]any{"intent_id": i.ID, "state": senderState(i), "handshake": i.Handshake})
}
