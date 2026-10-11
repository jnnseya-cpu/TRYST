// Package notify is TRYST's notification service (02 FR-026, FR-119; 03 §5 A1, §6.7;
// D-55): a masked in-app inbox, member settings, and content-free Web Push.
//
//   - Inbox notices say what happened without names, photos or excerpts.
//   - Push is off by default. When a member turns it on, pushes carry no payload; the device
//     shows the member's own neutral wording (default "You have an update").
//   - No email address is held in clear anywhere in TRYST, so there is no TRYST email about
//     activity. Billing emails (renewal reminders, receipts) go from the payment processor
//     to the billing address the member chose at checkout.
package notify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/dsr"
	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/problem"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

// Kinds and their in-app text. In-app text is seen only inside the signed-in app.
var texts = map[string]string{
	"match":         "You have a new match.",
	"message":       "You have a new message.",
	"meet_proposed": "A meet was proposed.",
	"meet_update":   "A meet was confirmed or changed.",
	"safety":        "There's an update from our safety team. See your account.",
	"appeal":        "Your appeal has been decided. See your account.",
	"renewal":       "Your plan renews soon. You can cancel any time in Plans and Keys.",
	"plan_ended":    "Your plan has ended.",
	"new_device":    "A new device signed in to your account. If this wasn't you, use Burn.",
	"payment":       "Payment received.",
}

const (
	maxNotices    = 100
	maxPushSubs   = 5
	DefaultText   = "You have an update"
	maxCustomText = 40
	// Coalescing: at most one push per member per window, so a busy conversation doesn't
	// light up a lock screen repeatedly.
	pushWindow = 2 * time.Minute
)

// Notice is one inbox item.
type Notice struct {
	ID   string    `json:"id"`
	Kind string    `json:"kind"`
	Text string    `json:"text"`
	Link string    `json:"link,omitempty"` // in-app path, never containing names
	At   time.Time `json:"at"`
	Read bool      `json:"read"`
}

// Settings are the member's notification choices.
type Settings struct {
	Push bool   `json:"push"`
	Text string `json:"text"`
}

type box struct {
	Notices  []Notice  `json:"notices"`
	Settings Settings  `json:"settings"`
	Subs     []string  `json:"push_endpoints"`
	LastPush time.Time `json:"last_push"`
}

// Pusher delivers a content-free push to one endpoint.
type Pusher interface {
	Send(ctx context.Context, endpoint string) error
	ValidEndpoint(endpoint string) bool
	PublicKey() string
}

// MemberAuth is satisfied by *identity.Server.
type MemberAuth interface {
	Authenticate(r *http.Request) (identity.Account, bool)
}

// Service is the HTTP handler and the Notify API for other services.
type Service struct {
	members MemberAuth
	st      store.Store
	push    Pusher
	now     func() time.Time
	mu      sync.Mutex
	boxes   map[string]*box
	mux     *http.ServeMux
	wg      sync.WaitGroup
}

func New(members MemberAuth, st store.Store, push Pusher) (*Service, error) {
	s := &Service{members: members, st: st, push: push, now: time.Now, boxes: map[string]*box{}, mux: http.NewServeMux()}
	docs, err := st.All(context.Background(), "inbox")
	if err != nil {
		return nil, err
	}
	for id, b := range docs {
		var bx box
		if json.Unmarshal(b, &bx) == nil {
			s.boxes[id] = &bx
		}
	}
	m := s.mux
	m.HandleFunc("GET /v1/notices", s.handleList)
	m.HandleFunc("POST /v1/notices/read", s.handleRead)
	m.HandleFunc("GET /v1/notifications/settings", s.handleGetSettings)
	m.HandleFunc("PUT /v1/notifications/settings", s.handlePutSettings)
	m.HandleFunc("PUT /v1/notifications/push", s.handleSubscribe)
	m.HandleFunc("DELETE /v1/notifications/push", s.handleUnsubscribe)
	return s, nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	s.mux.ServeHTTP(w, r)
}

func fail(w http.ResponseWriter, status int, code, title string) {
	problem.Write(w, problem.Problem{Title: title, Status: status, Code: code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// boxFor returns the member's inbox. Caller holds s.mu.
func (s *Service) boxFor(account string) *box {
	b, ok := s.boxes[account]
	if !ok {
		b = &box{Settings: Settings{Text: DefaultText}}
		s.boxes[account] = b
	}
	return b
}

// save persists one inbox. Caller holds s.mu.
func (s *Service) save(ctx context.Context, account string) {
	b, _ := json.Marshal(s.boxes[account])
	if err := s.st.Put(ctx, "inbox", account, b); err != nil {
		log.Printf("persist inbox failed: %v", err)
	}
}

// Notify records a notice and, if the member turned push on, sends a content-free push.
// Unknown kinds are ignored. Safe to call from any service, holding any lock: delivery is
// asynchronous.
func (s *Service) Notify(account, kind, link string) {
	text, ok := texts[kind]
	if !ok || account == "" {
		return
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	s.mu.Lock()
	bx := s.boxFor(account)
	now := s.now()
	bx.Notices = append([]Notice{{ID: "nt_" + hex.EncodeToString(b), Kind: kind, Text: text, Link: link, At: now}}, bx.Notices...)
	if len(bx.Notices) > maxNotices {
		bx.Notices = bx.Notices[:maxNotices]
	}
	var targets []string
	if bx.Settings.Push && s.push != nil && now.Sub(bx.LastPush) >= pushWindow {
		targets = append(targets, bx.Subs...)
		if len(targets) > 0 {
			bx.LastPush = now
		}
	}
	s.save(context.Background(), account)
	s.mu.Unlock()
	if len(targets) == 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		for _, ep := range targets {
			if err := s.push.Send(ctx, ep); err == ErrGone {
				s.dropEndpoint(account, ep)
			} else if err != nil {
				log.Printf("push failed: %v", err) // no endpoint or account logged
			}
		}
	}()
}

// Wait blocks until pending pushes finish (tests, shutdown).
func (s *Service) Wait() { s.wg.Wait() }

func (s *Service) dropEndpoint(account, ep string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bx := s.boxFor(account)
	kept := bx.Subs[:0]
	for _, e := range bx.Subs {
		if e != ep {
			kept = append(kept, e)
		}
	}
	bx.Subs = kept
	s.save(context.Background(), account)
}

func (s *Service) member(w http.ResponseWriter, r *http.Request) (string, bool) {
	acc, ok := s.members.Authenticate(r)
	if !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
	}
	return acc.ID, ok
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.member(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bx := s.boxFor(acc)
	unread := 0
	for _, n := range bx.Notices {
		if !n.Read {
			unread++
		}
	}
	list := append([]Notice{}, bx.Notices...)
	sort.SliceStable(list, func(i, j int) bool { return list[i].At.After(list[j].At) })
	writeJSON(w, 200, map[string]any{"notices": list, "unread": unread})
}

func (s *Service) handleRead(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.member(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bx := s.boxFor(acc)
	for i := range bx.Notices {
		bx.Notices[i].Read = true
	}
	s.save(r.Context(), acc)
	writeJSON(w, 200, map[string]any{"unread": 0})
}

func (s *Service) settingsView(bx *box) map[string]any {
	key := ""
	if s.push != nil {
		key = s.push.PublicKey()
	}
	return map[string]any{"push": bx.Settings.Push, "text": bx.Settings.Text, "devices": len(bx.Subs), "vapid_public_key": key, "push_available": s.push != nil}
}

func (s *Service) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.member(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, 200, s.settingsView(s.boxFor(acc)))
}

// neutralText accepts member wording only if it can't identify the service: short, no
// links, and never "TRYST".
func neutralText(t string) (string, bool) {
	t = strings.TrimSpace(t)
	if t == "" {
		return DefaultText, true
	}
	l := strings.ToLower(t)
	if len([]rune(t)) > maxCustomText || strings.Contains(l, "tryst") || strings.Contains(l, "http") || strings.Contains(l, "www.") {
		return "", false
	}
	return t, true
}

func (s *Service) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.member(w, r)
	if !ok {
		return
	}
	var in struct {
		Push *bool  `json:"push"`
		Text string `json:"text"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in) != nil {
		fail(w, 422, "validation_failed", "Invalid settings.")
		return
	}
	text, ok := neutralText(in.Text)
	if !ok {
		fail(w, 422, "validation_failed", "Use up to 40 characters that don't mention the app or include a link.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bx := s.boxFor(acc)
	if in.Push != nil {
		bx.Settings.Push = *in.Push
		if !*in.Push {
			bx.Subs = nil // turning push off forgets every device's endpoint
		}
	}
	bx.Settings.Text = text
	s.save(r.Context(), acc)
	writeJSON(w, 200, s.settingsView(bx))
}

func (s *Service) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.member(w, r)
	if !ok {
		return
	}
	var in struct {
		Endpoint string `json:"endpoint"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in)
	if s.push == nil || !s.push.ValidEndpoint(in.Endpoint) {
		fail(w, 422, "validation_failed", "This browser's push service isn't supported.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bx := s.boxFor(acc)
	if !bx.Settings.Push {
		fail(w, 409, "push_off", "Turn notifications on first.")
		return
	}
	for _, e := range bx.Subs {
		if e == in.Endpoint {
			writeJSON(w, 200, s.settingsView(bx))
			return
		}
	}
	if len(bx.Subs) >= maxPushSubs {
		bx.Subs = bx.Subs[1:]
	}
	bx.Subs = append(bx.Subs, in.Endpoint)
	s.save(r.Context(), acc)
	writeJSON(w, 200, s.settingsView(bx))
}

func (s *Service) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.member(w, r)
	if !ok {
		return
	}
	var in struct {
		Endpoint string `json:"endpoint"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in)
	s.mu.Lock()
	bx := s.boxFor(acc)
	s.mu.Unlock()
	s.dropEndpoint(acc, in.Endpoint)
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, 200, s.settingsView(bx))
}

// --- data rights ---

func (s *Service) Name() string { return "notifications" }

func (s *Service) Export(_ context.Context, account string) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bx, ok := s.boxes[account]
	if !ok {
		return nil, nil
	}
	return map[string]any{"notices": bx.Notices, "settings": bx.Settings, "push_devices": len(bx.Subs)}, nil
}

func (s *Service) Erase(ctx context.Context, account string) (dsr.Receipt, error) {
	s.mu.Lock()
	delete(s.boxes, account)
	s.mu.Unlock()
	if err := s.st.Delete(ctx, "inbox", account); err != nil {
		return dsr.Receipt{}, err
	}
	return dsr.Receipt{Part: "notifications", Erased: 1}, nil
}

var _ dsr.Part = (*Service)(nil)
