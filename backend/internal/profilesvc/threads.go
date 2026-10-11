package profilesvc

// Threads: end-to-end encrypted conversations (02 §5.7; FR-016, FR-114; D-51).
//
// A thread opens only when interest is mutual (Tonight today; Envoy handshakes from P2).
// The server relays ciphertext and never holds a key that can read it:
//
//   - every browser or phone registers a device public key (ECDH P-256); the private key is
//     created non-extractable on the device and never leaves it;
//   - a message is encrypted separately for every device of both members, each with a fresh
//     ephemeral key (ECDH -> HKDF-SHA256 -> AES-256-GCM, bound to thread, message and device);
//   - the server stores envelopes with an expiry from the thread's retention (7/30/90 days,
//     shortest wins) and deletes them when they expire;
//   - on-device Guardian signals (score and reason only) can freeze a thread for review;
//   - either member can block: the thread closes for both, and they never see each other
//     again in Tonight or discovery.
//
// MLS (RFC 9420) replaces the per-device scheme in P2 for forward secrecy and group threads.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"
)

const (
	maxDevices      = 5
	maxEnvelopeSize = 16 << 10 // per recipient device
	maxMessageBody  = 128 << 10
	defaultRetain   = 30
	maxPerFetch     = 200
)

var allowedRetention = map[int]bool{7: true, 30: true, 90: true}

type deviceKey struct {
	ID    string          `json:"id"`
	JWK   json.RawMessage `json:"jwk"`
	Added time.Time       `json:"added"`
}

type thread struct {
	ID        string            `json:"id"`
	Members   [2]string         `json:"members"`
	Created   time.Time         `json:"created"`
	Retention map[string]int    `json:"retention"` // per member; the shortest applies
	State     string            `json:"state"`     // normal | frozen | closed
	Seq       int64             `json:"seq"`
	LastAt    time.Time         `json:"last_at"`
	Origin    string            `json:"origin"` // tonight | handshake
	Signals   map[string]string `json:"signals,omitempty"`
}

type envelope struct {
	Device string `json:"device_id"`
	EPK    string `json:"epk"` // sender's ephemeral public key (base64 raw)
	IV     string `json:"iv"`
	CT     string `json:"ct"`
}

type message struct {
	Seq          int64      `json:"seq"`
	Sender       string     `json:"sender"`
	SenderDevice string     `json:"sender_device"`
	ClientID     string     `json:"client_id"`
	At           time.Time  `json:"at"`
	Expires      time.Time  `json:"expires"`
	Envelopes    []envelope `json:"envelopes"`
}

func (t *thread) other(me string) string {
	if t.Members[0] == me {
		return t.Members[1]
	}
	return t.Members[0]
}

func (t *thread) has(me string) bool { return t.Members[0] == me || t.Members[1] == me }

// retention is the shortest period any member chose, or 30 days.
func (t *thread) retention() int {
	d := 0
	for _, v := range t.Retention {
		if d == 0 || v < d {
			d = v
		}
	}
	if d == 0 {
		return defaultRetain
	}
	return d
}

func threadID(a, b string) string {
	ids := []string{a, b}
	sort.Strings(ids)
	h := sha256.Sum256([]byte("tryst/thread/v1|" + ids[0] + "|" + ids[1]))
	return "th_" + hex.EncodeToString(h[:12])
}

// threadState is the in-memory index; documents are the source of truth on restart.
type threadState struct {
	threads map[string]*thread         // id -> thread
	devices map[string][]deviceKey     // account -> device keys
	blocked map[string]map[string]bool // account -> accounts they blocked
}

func newThreadState() *threadState {
	return &threadState{threads: map[string]*thread{}, devices: map[string][]deviceKey{}, blocked: map[string]map[string]bool{}}
}

func (s *Service) loadThreads(ctx context.Context) error {
	docs, err := s.st.All(ctx, "thread")
	if err != nil {
		return err
	}
	for id, b := range docs {
		var t thread
		if err := json.Unmarshal(b, &t); err != nil {
			return fmt.Errorf("thread %s: %w", id, err)
		}
		s.th.threads[t.ID] = &t
	}
	devs, err := s.st.All(ctx, "devicekeys")
	if err != nil {
		return err
	}
	for acc, b := range devs {
		var d []deviceKey
		if json.Unmarshal(b, &d) == nil {
			s.th.devices[acc] = d
		}
	}
	blocks, err := s.st.All(ctx, "blocks")
	if err != nil {
		return err
	}
	for acc, b := range blocks {
		var m map[string]bool
		if json.Unmarshal(b, &m) == nil {
			s.th.blocked[acc] = m
		}
	}
	return nil
}

func (s *Service) saveThread(ctx context.Context, t *thread) {
	b, _ := json.Marshal(t)
	if err := s.st.Put(ctx, "thread", t.ID, b); err != nil {
		log.Printf("persist thread failed: %v", err)
	}
}

// isBlocked reports whether either member blocked the other, or either has been suspended
// or removed (their conversations close).
func (s *Service) isBlocked(a, b string) bool {
	return s.th.blocked[a][b] || s.th.blocked[b][a] || removed(s.standing[a]) || removed(s.standing[b])
}

// ensureThread opens the thread for a mutual pair. Caller holds s.mu.
func (s *Service) ensureThread(ctx context.Context, a, b, origin string) *thread {
	id := threadID(a, b)
	if t, ok := s.th.threads[id]; ok {
		return t
	}
	t := &thread{ID: id, Members: [2]string{a, b}, Created: s.now(), Retention: map[string]int{}, State: "normal", Origin: origin}
	s.th.threads[id] = t
	s.saveThread(ctx, t)
	return t
}

// --- handlers ---

func (s *Service) threadFor(w http.ResponseWriter, me, id string) *thread {
	t, ok := s.th.threads[id]
	if !ok || !t.has(me) {
		fail(w, 404, "not_found", "Not found")
		return nil
	}
	return t
}

func (s *Service) handleDeviceKey(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		DeviceID string          `json:"device_id"`
		JWK      json.RawMessage `json:"public_jwk"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in); err != nil || len(in.DeviceID) < 8 || len(in.DeviceID) > 64 || !validPublicJWK(in.JWK) {
		fail(w, 422, "validation_failed", "That device key isn't valid.")
		return
	}
	devs := s.th.devices[acc.ID]
	for i, d := range devs {
		if d.ID == in.DeviceID {
			devs[i].JWK = in.JWK
			s.saveDevices(r.Context(), acc.ID)
			writeJSON(w, 200, map[string]any{"devices": len(devs)})
			return
		}
	}
	if len(devs) >= maxDevices {
		devs = devs[1:] // oldest device drops out; it can no longer read new messages
	}
	s.th.devices[acc.ID] = append(devs, deviceKey{ID: in.DeviceID, JWK: in.JWK, Added: s.now()})
	s.saveDevices(r.Context(), acc.ID)
	writeJSON(w, 201, map[string]any{"devices": len(s.th.devices[acc.ID])})
}

func (s *Service) saveDevices(ctx context.Context, acc string) {
	b, _ := json.Marshal(s.th.devices[acc])
	if err := s.st.Put(ctx, "devicekeys", acc, b); err != nil {
		log.Printf("persist device keys failed: %v", err)
	}
}

// validPublicJWK accepts only an EC P-256 public key (no private part).
func validPublicJWK(raw json.RawMessage) bool {
	var k map[string]any
	if json.Unmarshal(raw, &k) != nil || len(raw) > 1024 {
		return false
	}
	_, hasD := k["d"]
	return k["kty"] == "EC" && k["crv"] == "P-256" && k["x"] != nil && k["y"] != nil && !hasD
}

type threadView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Age       *int      `json:"age"`
	State     string    `json:"state"`
	Retention int       `json:"retention_days"`
	LastSeq   int64     `json:"last_seq"`
	LastAt    time.Time `json:"last_at"`
	Created   time.Time `json:"created"`
}

func (s *Service) viewThread(me string, t *thread) threadView {
	other := s.data[t.other(me)]
	v := threadView{ID: t.ID, State: t.State, Retention: t.retention(), LastSeq: t.Seq, LastAt: t.LastAt, Created: t.Created}
	if other != nil {
		v.Name, v.Age = other.DisplayName, s.ageOf(other)
	}
	if t.State == "normal" && s.isBlocked(t.Members[0], t.Members[1]) {
		v.State = "closed"
	}
	return v
}

func (s *Service) handleThreads(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	out := []threadView{}
	for _, t := range s.th.threads {
		if t.has(acc.ID) {
			out = append(out, s.viewThread(acc.ID, t))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastAt.After(out[j].LastAt) })
	writeJSON(w, 200, map[string]any{"threads": out})
}

func (s *Service) handleThread(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	t := s.threadFor(w, acc.ID, r.PathValue("id"))
	if t == nil {
		return
	}
	other := t.other(acc.ID)
	writeJSON(w, 200, map[string]any{
		"thread":        s.viewThread(acc.ID, t),
		"my_devices":    s.th.devices[acc.ID],
		"their_devices": s.th.devices[other],
	})
}

func (s *Service) handleSend(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	t := s.threadFor(w, acc.ID, r.PathValue("id"))
	if t == nil {
		return
	}
	if t.State != "normal" || s.isBlocked(t.Members[0], t.Members[1]) {
		fail(w, 409, "thread_closed", "This conversation is closed.")
		return
	}
	var in struct {
		ClientID     string     `json:"client_id"`
		SenderDevice string     `json:"sender_device"`
		Envelopes    []envelope `json:"envelopes"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxMessageBody)).Decode(&in); err != nil || in.ClientID == "" || len(in.Envelopes) == 0 {
		fail(w, 422, "validation_failed", "That message couldn't be sent.")
		return
	}
	// Envelopes only for devices in this thread, at most one each, sender's device must be theirs.
	allowed := map[string]bool{}
	for _, m := range t.Members {
		for _, d := range s.th.devices[m] {
			allowed[d.ID] = true
		}
	}
	mine := false
	for _, d := range s.th.devices[acc.ID] {
		mine = mine || d.ID == in.SenderDevice
	}
	if !mine {
		fail(w, 422, "validation_failed", "Register this device before sending.")
		return
	}
	seen := map[string]bool{}
	for _, e := range in.Envelopes {
		if !allowed[e.Device] || seen[e.Device] || len(e.CT) > maxEnvelopeSize || e.EPK == "" || e.IV == "" {
			fail(w, 422, "validation_failed", "That message couldn't be sent.")
			return
		}
		seen[e.Device] = true
	}
	// Idempotent on client id (retries never duplicate a message).
	key := "msg:" + t.ID
	if prev, err := s.st.Get(r.Context(), key+":client", acc.ID+"|"+in.ClientID); err == nil {
		writeJSON(w, 200, map[string]any{"seq": json.RawMessage(prev)})
		return
	}
	t.Seq++
	now := s.now()
	m := message{Seq: t.Seq, Sender: acc.ID, SenderDevice: in.SenderDevice, ClientID: in.ClientID, At: now,
		Expires: now.AddDate(0, 0, t.retention()), Envelopes: in.Envelopes}
	b, _ := json.Marshal(m)
	if err := s.st.Put(r.Context(), key, fmt.Sprintf("%012d", m.Seq), b); err != nil {
		t.Seq--
		fail(w, 503, "storage_unavailable", "That message couldn't be sent. Try again.")
		return
	}
	_ = s.st.Put(r.Context(), key+":client", acc.ID+"|"+in.ClientID, []byte(strconv.FormatInt(m.Seq, 10)))
	t.LastAt = now
	s.saveThread(r.Context(), t)
	writeJSON(w, 201, map[string]any{"seq": m.Seq})
}

func (s *Service) handleMessages(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	t := s.threadFor(w, acc.ID, r.PathValue("id"))
	if t == nil {
		s.mu.Unlock()
		return
	}
	device := r.URL.Query().Get("device_id")
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	tid := t.ID
	s.mu.Unlock()

	docs, err := s.st.All(r.Context(), "msg:"+tid)
	if err != nil {
		fail(w, 503, "storage_unavailable", "Try again.")
		return
	}
	now := s.now()
	type out struct {
		Seq      int64     `json:"seq"`
		ClientID string    `json:"client_id"`
		Mine     bool      `json:"mine"`
		At       time.Time `json:"at"`
		Expires  time.Time `json:"expires"`
		Envelope *envelope `json:"envelope"`
	}
	var msgs []out
	for id, b := range docs {
		var m message
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		if now.After(m.Expires) {
			_ = s.st.Delete(r.Context(), "msg:"+tid, id) // retention: purge on sight (and by the sweep job)
			continue
		}
		if m.Seq <= after {
			continue
		}
		o := out{Seq: m.Seq, ClientID: m.ClientID, Mine: m.Sender == acc.ID, At: m.At, Expires: m.Expires}
		for i := range m.Envelopes {
			if m.Envelopes[i].Device == device {
				o.Envelope = &m.Envelopes[i]
			}
		}
		msgs = append(msgs, o)
	}
	sort.Slice(msgs, func(i, j int) bool { return msgs[i].Seq < msgs[j].Seq })
	if len(msgs) > maxPerFetch {
		msgs = msgs[:maxPerFetch]
	}
	if msgs == nil {
		msgs = []out{}
	}
	writeJSON(w, 200, map[string]any{"messages": msgs})
}

func (s *Service) handleRetention(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	t := s.threadFor(w, acc.ID, r.PathValue("id"))
	if t == nil {
		return
	}
	var in struct {
		Days int `json:"days"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in) != nil || !allowedRetention[in.Days] {
		fail(w, 422, "validation_failed", "Choose 7, 30 or 90 days.")
		return
	}
	t.Retention[acc.ID] = in.Days
	s.saveThread(r.Context(), t)
	writeJSON(w, 200, map[string]any{"thread": s.viewThread(acc.ID, t)})
}

// handleBlock closes the thread for both and hides the pair from each other everywhere.
// The other member is never told why.
func (s *Service) handleBlock(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	t := s.threadFor(w, acc.ID, r.PathValue("id"))
	if t == nil {
		return
	}
	s.block(r.Context(), acc.ID, t.other(acc.ID))
	t.State = "closed"
	s.saveThread(r.Context(), t)
	writeJSON(w, 200, map[string]any{"thread": s.viewThread(acc.ID, t)})
}

func (s *Service) block(ctx context.Context, me, them string) {
	if s.th.blocked[me] == nil {
		s.th.blocked[me] = map[string]bool{}
	}
	s.th.blocked[me][them] = true
	b, _ := json.Marshal(s.th.blocked[me])
	if err := s.st.Put(ctx, "blocks", me, b); err != nil {
		log.Printf("persist blocks failed: %v", err)
	}
}

// handleSignal receives an on-device Guardian signal (FR-102, FR-110): kind and model version
// only, never message content. Child-safety and extortion signals freeze the thread for
// review; fraud signals are counted for cross-account patterns.
func (s *Service) handleSignal(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	t := s.threadFor(w, acc.ID, r.PathValue("id"))
	if t == nil {
		return
	}
	var in struct {
		Kind  string `json:"kind"`
		Model string `json:"model"`
		About string `json:"about"` // "received" or "blocked_send"
	}
	if json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in) != nil || (in.About != "received" && in.About != "blocked_send") {
		fail(w, 422, "validation_failed", "Invalid signal.")
		return
	}
	switch in.Kind {
	case "child_safety", "extortion":
		t.State = "frozen"
	case "fraud":
	default:
		fail(w, 422, "validation_failed", "Invalid signal.")
		return
	}
	if t.Signals == nil {
		t.Signals = map[string]string{}
	}
	t.Signals[fmt.Sprintf("%d:%s", s.now().Unix(), acc.ID[:min(8, len(acc.ID))])] = in.Kind + "/" + in.Model + "/" + in.About
	s.saveThread(r.Context(), t)
	if s.OnSignal != nil {
		// "received": the other member sent it. "blocked_send": the caller's own device
		// refused to send it, so the caller is the subject.
		subject := t.other(acc.ID)
		if in.About == "blocked_send" {
			subject = acc.ID
		}
		s.OnSignal(t.ID, acc.ID, subject, in.Kind)
	}
	writeJSON(w, 200, map[string]any{"thread": s.viewThread(acc.ID, t)})
}

func (s *Service) handleThreadPhoto(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	t := s.threadFor(w, acc.ID, r.PathValue("id"))
	if t == nil {
		return
	}
	otherID := t.other(acc.ID)
	other := s.data[otherID]
	if other == nil || s.isBlocked(acc.ID, otherID) {
		fail(w, 404, "not_found", "Not found")
		return
	}
	shown, _ := s.visiblePhotos(other)
	if len(shown) == 0 {
		fail(w, 404, "not_found", "Not found")
		return
	}
	data, err := s.photoBytes(r.Context(), otherID, shown[0])
	if err != nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	w.Header().Set("Content-Type", shown[0].ContentType)
	_, _ = w.Write(data)
}
