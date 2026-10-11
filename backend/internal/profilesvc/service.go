// Package profilesvc is the profile and photo API (docs/spec/02_Shared_Contracts.md §5.3;
// FR-092, FR-093, FR-098). In production it is the `profile` module of core-svc; in
// development it is mounted beside identity-svc so the web app can use it end to end.
//
// Every upload is checked before anyone can see it: provenance (AI labels), the moderation
// vendors (detector, reverse-image, CSAM, faces), then the rules in internal/profile.
// Rejected images are never stored. Accepted images are decoded and re-encoded, which drops
// every metadata block (location, device, provenance) before storage.
package profilesvc

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/problem"
	"github.com/jnnseya-cpu/tryst/backend/internal/profile"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

const (
	MaxUploadBytes = 8 << 20 // 8 MB per photo
	MaxPixels      = 40_000_000
	maxJSON        = 16 << 10
)

// Authenticator is satisfied by *identity.Server.
type Authenticator interface {
	Authenticate(r *http.Request) (identity.Account, bool)
}

type photo struct {
	ID          string
	Main        bool
	Revealed    bool
	Outcome     profile.Outcome
	Reason      string
	MainOK      bool // passes the main-photo rules (face clearly matched, not explicit)
	MainReason  string
	ContentType string
	Data        []byte `json:"-"` // sealed in the blob store under the member's key; never in the document
	Added       time.Time
}

type memberProfile struct {
	BirthYM     string // copied from the signed-in account; shown as age only
	Tier        string
	DisplayName string
	AreaID      string
	CityOnly    bool
	Photos      []*photo // gallery order
}

// Service is the HTTP handler.
type Service struct {
	auth Authenticator
	mod  Moderator
	now  func() time.Time
	mu   sync.Mutex
	data map[string]*memberProfile
	mux  *http.ServeMux
	tn   *tonight
	// Eligible is the Stage 0 check (production: Broker gates). Nil = dev default.
	Eligible Eligible
	// Persistence (D-50): profile documents and sealed photo blobs in the profile database;
	// per-member photo keys in the key vault (crypto-erasure).
	st    store.Store
	vault *store.Vault
	dirty map[string]bool
	th    *threadState
	mt    *meetState
	// Alerts delivers safety escalations (trusted contact, safety line, emergency).
	Alerts     Alerting
	alertWG    sync.WaitGroup
	alertRetry time.Duration
	// OnSignal is called when an on-device Guardian signal arrives (safety case queue).
	OnSignal func(threadID, reporter, subject, kind string)
	// standing holds enforcement outcomes other than good (see standing.go).
	standing map[string]string
	// Notify records a masked notice for a member (notification service). May be nil.
	Notify func(account, kind, link string)
	// MinDiscoveryTier is the tier needed to use Tonight (V2 in production, E-01).
	MinDiscoveryTier string
}

// New returns the service. env "prod" refuses DevModerator.
// New returns an in-memory service (tests, quick local runs).
func New(auth Authenticator, mod Moderator, env string) (*Service, error) {
	kek := make([]byte, 32)
	_, _ = rand.Read(kek)
	vault, _ := store.NewVault(store.NewMemory(), kek)
	return NewWithStore(auth, mod, env, store.NewMemory(), vault)
}

// NewWithStore returns a service that loads and saves through st, sealing photos with vault.
func NewWithStore(auth Authenticator, mod Moderator, env string, st store.Store, vault *store.Vault) (*Service, error) {
	if mod == nil {
		mod = NoModerator{}
	}
	if _, dev := mod.(DevModerator); dev && env != "dev" {
		return nil, errors.New("DevModerator is forbidden outside the dev environment")
	}
	s := &Service{auth: auth, mod: mod, now: time.Now, data: map[string]*memberProfile{}, mux: http.NewServeMux(),
		tn: newTonight(), MinDiscoveryTier: "V2", st: st, vault: vault, dirty: map[string]bool{}, th: newThreadState(), mt: newMeetState(), standing: map[string]string{}, alertRetry: 5 * time.Second}
	if err := s.load(context.Background()); err != nil {
		return nil, err
	}
	if err := s.loadThreads(context.Background()); err != nil {
		return nil, err
	}
	if err := s.loadMeets(context.Background()); err != nil {
		return nil, err
	}
	if err := s.loadStanding(context.Background()); err != nil {
		return nil, err
	}
	if env == "dev" {
		s.MinDiscoveryTier = "V0" // dev accounts have no real ID check; production requires V2
	}
	m := s.mux
	m.HandleFunc("GET /v1/profile", s.handleGet)
	m.HandleFunc("PUT /v1/profile", s.handlePut)
	m.HandleFunc("GET /v1/profile/areas", s.handleAreas)
	m.HandleFunc("GET /v1/profile/card", s.handleCard)
	m.HandleFunc("POST /v1/profile/photos", s.handleUpload)
	m.HandleFunc("PUT /v1/profile/photos/order", s.handleOrder)
	m.HandleFunc("GET /v1/profile/photos/{id}", s.handleImage)
	m.HandleFunc("PATCH /v1/profile/photos/{id}", s.handlePatch)
	m.HandleFunc("POST /v1/profile/photos/{id}/main", s.handleMain)
	m.HandleFunc("DELETE /v1/profile/photos/{id}", s.handleDelete)
	m.HandleFunc("PUT /v1/devices/key", s.handleDeviceKey)
	m.HandleFunc("GET /v1/threads", s.handleThreads)
	m.HandleFunc("GET /v1/threads/{id}", s.handleThread)
	m.HandleFunc("GET /v1/threads/{id}/messages", s.handleMessages)
	m.HandleFunc("POST /v1/threads/{id}/messages", s.handleSend)
	m.HandleFunc("PUT /v1/threads/{id}/retention", s.handleRetention)
	m.HandleFunc("POST /v1/threads/{id}/block", s.handleBlock)
	m.HandleFunc("POST /v1/threads/{id}/signal", s.handleSignal)
	m.HandleFunc("GET /v1/threads/{id}/photo", s.handleThreadPhoto)
	m.HandleFunc("POST /v1/threads/{id}/meets", s.handleProposeMeet)
	m.HandleFunc("GET /v1/meets/{meet}", s.handleGetMeet)
	m.HandleFunc("POST /v1/meets/{meet}/safety", s.handleSafetyStart)
	m.HandleFunc("POST /v1/meets/{meet}/checkin", s.handleCheckIn)
	m.HandleFunc("POST /v1/meets/{meet}/{decision}", s.handleMeetDecision)
	m.HandleFunc("POST /v1/safety/panic", s.handlePanic)
	m.HandleFunc("GET /v1/tonight", s.handleTonight)
	m.HandleFunc("PUT /v1/tonight", s.handleTonightStart)
	m.HandleFunc("DELETE /v1/tonight", s.handleTonightStop)
	m.HandleFunc("POST /v1/tonight/interest/{handle}", s.handleTonightInterest)
	m.HandleFunc("GET /v1/tonight/photos/{handle}/{n}", s.handleTonightPhoto)
	return s, nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	s.mux.ServeHTTP(w, r)
	if r.Method != http.MethodGet {
		s.flush(r.Context())
	}
}

func (s *Service) load(ctx context.Context) error {
	docs, err := s.st.All(ctx, "profile")
	if err != nil {
		return err
	}
	for id, b := range docs {
		var p memberProfile
		if err := json.Unmarshal(b, &p); err != nil {
			return err
		}
		s.data[id] = &p
	}
	return nil
}

// flush saves profiles changed by the request.
func (s *Service) flush(ctx context.Context) {
	s.mu.Lock()
	docs := map[string][]byte{}
	for id := range s.dirty {
		if p, ok := s.data[id]; ok {
			docs[id], _ = json.Marshal(p)
		}
	}
	s.dirty = map[string]bool{}
	s.mu.Unlock()
	for id, b := range docs {
		if err := s.st.Put(ctx, "profile", id, b); err != nil {
			log.Printf("persist profile failed: %v", err)
		}
	}
}

func (s *Service) notify(account, kind, link string) {
	if s.Notify != nil {
		s.Notify(account, kind, link)
	}
}

func blobID(account, photoID string) string { return "photo/" + account + "/" + photoID }

// photoBytes loads and opens a stored photo.
func (s *Service) photoBytes(ctx context.Context, account string, ph *photo) ([]byte, error) {
	ct, err := s.st.GetBlob(ctx, blobID(account, ph.ID))
	if err != nil {
		return nil, err
	}
	return s.vault.Open(ctx, account, ct, []byte("photo:"+ph.ID))
}

func fail(w http.ResponseWriter, status int, code, title string) {
	problem.Write(w, problem.Problem{Title: title, Status: status, Code: code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// member authenticates and returns the caller's profile, created on first use.
func (s *Service) member(w http.ResponseWriter, r *http.Request) (identity.Account, *memberProfile, bool) {
	acc, ok := s.auth.Authenticate(r)
	if !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
		return acc, nil, false
	}
	s.mu.Lock()
	if removed(s.standing[acc.ID]) {
		s.mu.Unlock()
		fail(w, 403, "account_suspended", "Your account is suspended. You can see why and appeal from your account page.")
		return acc, nil, false
	}
	p, ok := s.data[acc.ID]
	if !ok {
		p = &memberProfile{}
		s.data[acc.ID] = p
	}
	p.BirthYM, p.Tier = acc.BirthYM, acc.Tier.String()
	if r.Method != http.MethodGet {
		s.dirty[acc.ID] = true
	}
	return acc, p, true
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func find(p *memberProfile, id string) (int, *photo) {
	for i, ph := range p.Photos {
		if ph.ID == id {
			return i, ph
		}
	}
	return -1, nil
}

// --- views ---

type photoView struct {
	ID          string `json:"id"`
	Main        bool   `json:"main"`
	Revealed    bool   `json:"revealed"`
	Status      string `json:"status"` // live | checking
	Reason      string `json:"reason,omitempty"`
	CanBeMain   bool   `json:"can_be_main"`
	MainProblem string `json:"main_problem,omitempty"`
	URL         string `json:"url"`
}

func view(ph *photo) photoView {
	st := "live"
	if ph.Outcome == profile.Review {
		st = "checking"
	}
	return photoView{ID: ph.ID, Main: ph.Main, Revealed: ph.Revealed, Status: st, Reason: ph.Reason,
		CanBeMain: ph.MainOK && ph.Outcome == profile.Approve, MainProblem: ph.MainReason, URL: "/v1/profile/photos/" + ph.ID}
}

func gallery(p *memberProfile) []profile.Photo {
	out := make([]profile.Photo, 0, len(p.Photos))
	for _, ph := range p.Photos {
		out = append(out, profile.Photo{ID: ph.ID, Main: ph.Main, Outcome: ph.Outcome, Revealed: ph.Revealed})
	}
	return out
}

func (s *Service) age(acc identity.Account) *int {
	b, err := profile.ParseBirthYM(acc.BirthYM)
	if err != nil {
		return nil
	}
	a := profile.AgeYears(b, s.now())
	return &a
}

func (s *Service) ownerView(acc identity.Account, p *memberProfile) map[string]any {
	photos := make([]photoView, 0, len(p.Photos))
	for _, ph := range p.Photos {
		photos = append(photos, view(ph))
	}
	var problems []string
	if p.DisplayName == "" {
		problems = append(problems, "Choose a name to show other members.")
	}
	if p.AreaID == "" {
		problems = append(problems, "Choose your area.")
	}
	if err := profile.ValidateGallery(gallery(p)); err != nil {
		problems = append(problems, sentence(err))
	}
	area := map[string]any{}
	if a, ok := areaByID(p.AreaID); ok {
		area = map[string]any{"id": a.ID, "label": a.Label(p.CityOnly)}
	}
	return map[string]any{
		"display_name": p.DisplayName, "age": s.age(acc), "area": area, "city_only": p.CityOnly,
		"photos": photos, "max_photos": profile.MaxPhotos, "ready": len(problems) == 0, "problems": problems,
	}
}

func sentence(err error) string {
	switch {
	case errors.Is(err, profile.ErrNoMain):
		return "Add a main photo that clearly shows your face."
	case errors.Is(err, profile.ErrMainPending):
		return "Your main photo is still being checked."
	case errors.Is(err, profile.ErrMainHidden):
		return "Your main photo must be shown, not blurred."
	case errors.Is(err, profile.ErrTooMany):
		return "You can show at most 10 photos."
	}
	return err.Error()
}

// --- handlers ---

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	writeJSON(w, 200, s.ownerView(acc, p))
}

func (s *Service) handleAreas(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth.Authenticate(r); !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	writeJSON(w, 200, map[string]any{"areas": Areas()})
}

func (s *Service) handlePut(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		DisplayName *string `json:"display_name"`
		AreaID      *string `json:"area_id"`
		CityOnly    *bool   `json:"city_only"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in); err != nil {
		fail(w, 422, "validation_failed", "That didn't look right. Please try again.")
		return
	}
	if in.DisplayName != nil {
		name := *in.DisplayName
		if n := len([]rune(name)); n < 2 || n > 24 {
			fail(w, 422, "validation_failed", "Names are 2 to 24 characters.")
			return
		}
		if f := profile.ScreenText(name); len(f) > 0 {
			fail(w, 422, "text_refused", "Names can't include contact details, links, prices or anything about age. Please choose another.")
			return
		}
		p.DisplayName = name
	}
	if in.AreaID != nil {
		if _, ok := areaByID(*in.AreaID); !ok {
			fail(w, 422, "validation_failed", "Choose an area from the list.")
			return
		}
		p.AreaID = *in.AreaID
	}
	if in.CityOnly != nil {
		p.CityOnly = *in.CityOnly
	}
	writeJSON(w, 200, s.ownerView(acc, p))
}

// strip decodes and re-encodes the image, which removes every metadata block.
func strip(raw []byte) ([]byte, string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "jpeg" && format != "png") {
		return nil, "", errors.New("format")
	}
	if cfg.Width*cfg.Height > MaxPixels || cfg.Width < 200 || cfg.Height < 200 {
		return nil, "", errors.New("size")
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", errors.New("format")
	}
	var out bytes.Buffer
	if format == "png" {
		err = png.Encode(&out, img)
		return out.Bytes(), "image/png", err
	}
	err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 90})
	return out.Bytes(), "image/jpeg", err
}

func (s *Service) handleUpload(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.auth.Authenticate(r)
	if !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
		return
	}
	s.mu.Lock()
	suspended := removed(s.standing[acc.ID])
	s.mu.Unlock()
	if suspended {
		fail(w, 403, "account_suspended", "Your account is suspended. You can see why and appeal from your account page.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes+64<<10)
	if err := r.ParseMultipartForm(MaxUploadBytes); err != nil { // #nosec G120 -- body capped by MaxBytesReader on the line above
		fail(w, 413, "too_large", "Photos can be up to 8 MB.")
		return
	}
	f, _, err := r.FormFile("photo")
	if err != nil {
		fail(w, 422, "validation_failed", "Choose a photo to upload.")
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, MaxUploadBytes+1))
	if err != nil || len(raw) > MaxUploadBytes {
		fail(w, 413, "too_large", "Photos can be up to 8 MB.")
		return
	}

	s.mu.Lock()
	p := s.data[acc.ID]
	if p == nil {
		p = &memberProfile{}
		s.data[acc.ID] = p
	}
	full := len(p.Photos) >= profile.MaxPhotos
	s.mu.Unlock()
	if full {
		fail(w, 409, "gallery_full", "You already have 10 photos. Remove one to add another.")
		return
	}

	checks, err := s.mod.Check(context.Background(), acc.ID, raw)
	if err != nil {
		checks = profile.Checks{DetectorRan: false} // vendor error: fail closed to review
	}
	checks.Provenance = profile.ReadProvenance(raw)
	d := profile.Moderate(checks, 1, false)
	if d.Outcome == profile.Reject || d.Outcome == profile.Report {
		// Never stored. CSAM reporting is handled by the vendor pipeline (03 §7.3).
		fail(w, 422, "photo_refused", d.Reason)
		return
	}
	data, ctype, err := strip(raw)
	if err != nil {
		fail(w, 422, "photo_refused", "Please upload a JPEG or PNG photo at least 200 × 200 pixels.")
		return
	}
	mainD := profile.Moderate(checks, 1, true)
	phID := newID()
	sealed, err := s.vault.Seal(r.Context(), acc.ID, data, []byte("photo:"+phID))
	if err == nil {
		err = s.st.PutBlob(r.Context(), blobID(acc.ID, phID), sealed)
	}
	if err != nil {
		fail(w, 503, "storage_unavailable", "We couldn't save that photo. Please try again.")
		return
	}
	ph := &photo{ID: phID, Revealed: true, Outcome: d.Outcome, Reason: d.Reason, ContentType: ctype,
		MainOK: mainD.Outcome == profile.Approve, Added: s.now()}
	if !ph.MainOK && mainD.Outcome == profile.Reject {
		ph.MainReason = mainD.Reason
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(p.Photos) >= profile.MaxPhotos {
		fail(w, 409, "gallery_full", "You already have 10 photos. Remove one to add another.")
		return
	}
	s.dirty[acc.ID] = true
	hasMain := false
	for _, x := range p.Photos {
		hasMain = hasMain || x.Main
	}
	if !hasMain && ph.MainOK {
		ph.Main = true
		p.Photos = append([]*photo{ph}, p.Photos...)
	} else {
		p.Photos = append(p.Photos, ph)
	}
	writeJSON(w, 201, map[string]any{"photo": view(ph), "profile": s.ownerView(acc, p)})
}

func (s *Service) handleImage(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	_, ph := find(p, r.PathValue("id"))
	if ph == nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	w.Header().Set("Content-Type", ph.ContentType)
	w.Header().Set("Content-Disposition", "inline")
	data, err := s.photoBytes(r.Context(), acc.ID, ph)
	if err != nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	_, _ = w.Write(data) // #nosec G705 -- our own re-encoded image; type set by us, nosniff + sandbox CSP
}

func (s *Service) handlePatch(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	_, ph := find(p, r.PathValue("id"))
	if ph == nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	var in struct {
		Revealed *bool `json:"revealed"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in); err != nil || in.Revealed == nil {
		fail(w, 422, "validation_failed", "That didn't look right. Please try again.")
		return
	}
	if ph.Main && !*in.Revealed {
		fail(w, 422, "main_always_shown", "Your main photo is always shown. Choose another main photo first.")
		return
	}
	ph.Revealed = *in.Revealed
	writeJSON(w, 200, s.ownerView(acc, p))
}

func (s *Service) handleMain(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	i, ph := find(p, r.PathValue("id"))
	if ph == nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	if !ph.MainOK || ph.Outcome != profile.Approve {
		reason := ph.MainReason
		if reason == "" {
			reason = "This photo can't be your main photo until it has been checked."
		}
		fail(w, 422, "not_main_eligible", reason)
		return
	}
	for _, x := range p.Photos {
		x.Main = false
	}
	ph.Main, ph.Revealed = true, true
	p.Photos = append(p.Photos[:i], p.Photos[i+1:]...)
	p.Photos = append([]*photo{ph}, p.Photos...)
	writeJSON(w, 200, s.ownerView(acc, p))
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	i, ph := find(p, r.PathValue("id"))
	if ph == nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	p.Photos = append(p.Photos[:i], p.Photos[i+1:]...)
	if err := s.st.DeleteBlob(r.Context(), blobID(acc.ID, ph.ID)); err != nil {
		log.Printf("delete photo blob failed: %v", err)
	}
	writeJSON(w, 200, s.ownerView(acc, p))
}

func (s *Service) handleOrder(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in); err != nil || len(in.IDs) != len(p.Photos) {
		fail(w, 422, "validation_failed", "That order didn't match your photos.")
		return
	}
	pos := map[string]int{}
	for i, id := range in.IDs {
		pos[id] = i
	}
	for _, ph := range p.Photos {
		if _, ok := pos[ph.ID]; !ok {
			fail(w, 422, "validation_failed", "That order didn't match your photos.")
			return
		}
	}
	sort.SliceStable(p.Photos, func(a, b int) bool {
		// the main photo always stays first
		if p.Photos[a].Main != p.Photos[b].Main {
			return p.Photos[a].Main
		}
		return pos[p.Photos[a].ID] < pos[p.Photos[b].ID]
	})
	writeJSON(w, 200, s.ownerView(acc, p))
}

// handleCard returns the profile exactly as another member sees it: only approved photos,
// blurred ones marked as such, and an approximate distance for a viewer in the same area.
func (s *Service) handleCard(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	type cardPhoto struct {
		URL     string `json:"url"`
		Main    bool   `json:"main"`
		Blurred bool   `json:"blurred"`
	}
	var photos []cardPhoto
	for _, v := range profile.Visible(gallery(p)) {
		photos = append(photos, cardPhoto{URL: "/v1/profile/photos/" + v.ID, Main: v.Main, Blurred: !v.Revealed})
	}
	area, distance := "", ""
	if a, ok := areaByID(p.AreaID); ok {
		area = a.Label(p.CityOnly)
		distance, _ = profile.DistanceBand(a.Cell, a.Cell)
	}
	writeJSON(w, 200, map[string]any{
		"display_name": p.DisplayName, "age": s.age(acc), "area": area, "distance": distance,
		"photos": photos, "verified": acc.Tier.String(),
		"ready": profile.ValidateGallery(gallery(p)) == nil && p.DisplayName != "" && p.AreaID != "",
	})
}
