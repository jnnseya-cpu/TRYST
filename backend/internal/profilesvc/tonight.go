package profilesvc

// Tonight: "I'm free tonight, who nearby is too?" (FR-109, D-46; SPARK mode, 01 §5.2).
//
//   - Opt-in and time-boxed: 1 to 6 hours, then it switches itself off. Stop at any time.
//   - Reciprocal: you only see members who are also free tonight, and they only see you
//     while you are. Nobody can browse who is out tonight without being visible themselves.
//   - Only verified members (V2 in production) with a complete profile, inside the Stage 0
//     gates (blocks, ExclusionRing, partner shield, hard limits; Eligible below).
//   - Distance is the approximate band only; members are identified by per-viewer handles,
//     so nobody can track an account across lists.
//   - Interest stays private until it is mutual. At most 10 interests per evening.

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/profile"
)

const (
	MaxTonightHours     = 6
	MaxTonightInterests = 10
	maxTonightPeople    = 30
)

var allowedRadius = map[int]bool{5: true, 10: true, 25: true, 50: true}

type tonightEntry struct {
	until     time.Time
	radiusKm  int
	interests map[string]bool // member account ids I am interested in
}

type tonight struct {
	key     []byte
	entries map[string]*tonightEntry
	handles map[string]map[string]string // viewer → handle → member
}

func newTonight() *tonight {
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	return &tonight{key: k, entries: map[string]*tonightEntry{}, handles: map[string]map[string]string{}}
}

func (t *tonight) handle(viewer, member string) string {
	m := hmac.New(sha256.New, t.key)
	m.Write([]byte(viewer + "|" + member))
	h := hex.EncodeToString(m.Sum(nil))[:24]
	if t.handles[viewer] == nil {
		t.handles[viewer] = map[string]string{}
	}
	t.handles[viewer][h] = member
	return h
}

func (t *tonight) active(id string, now time.Time) *tonightEntry {
	e := t.entries[id]
	if e == nil || !now.Before(e.until) {
		if e != nil {
			delete(t.entries, id) // expired: switches itself off, interests go with it
		}
		return nil
	}
	return e
}

// Eligible is the Stage 0 check between two members. Production calls Broker's gates
// (agents/src/tryst_agents/broker/gates.py: tier, hard limits, ExclusionRing, partner_secret,
// blocks, safety state, jurisdiction). The dev default only excludes the member themself.
type Eligible func(viewer, member string) bool

func (s *Service) eligible(viewer, member string) bool {
	if viewer == member || s.isBlocked(viewer, member) {
		return false
	}
	if s.Eligible != nil {
		return s.Eligible(viewer, member)
	}
	return true
}

func (s *Service) ready(p *memberProfile) bool {
	return p.DisplayName != "" && p.AreaID != "" && profile.ValidateGallery(gallery(p)) == nil
}

type tonightPerson struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	Age         *int   `json:"age"`
	Area        string `json:"area"`
	Distance    string `json:"distance"`
	Photos      int    `json:"photos"` // photos this viewer may see now (main + shown)
	Blurred     int    `json:"blurred"`
	Interested  bool   `json:"interested"`
	Mutual      bool   `json:"mutual"`
	ThreadID    string `json:"thread_id,omitempty"`
	km          float64
}

func (s *Service) visiblePhotos(p *memberProfile) (shown []*photo, blurred int) {
	for _, ph := range p.Photos {
		if ph.Outcome != profile.Approve {
			continue
		}
		if ph.Main || ph.Revealed {
			shown = append(shown, ph)
		} else {
			blurred++
		}
	}
	return shown, blurred
}

func (s *Service) ageOf(p *memberProfile) *int {
	b, err := profile.ParseBirthYM(p.BirthYM)
	if err != nil {
		return nil
	}
	a := profile.AgeYears(b, s.now())
	return &a
}

// tonightView builds the caller's view. Caller holds s.mu.
func (s *Service) tonightView(me string, p *memberProfile) map[string]any {
	now := s.now()
	e := s.tn.active(me, now)
	out := map[string]any{"on": e != nil, "ready": s.ready(p), "tier_ok": p.Tier >= s.MinDiscoveryTier,
		"max_hours": MaxTonightHours, "people": []tonightPerson{}, "mutual": []tonightPerson{}}
	if e == nil {
		return out
	}
	out["until"] = e.until.UTC().Format(time.RFC3339)
	out["radius_km"] = e.radiusKm
	out["interests_left"] = MaxTonightInterests - len(e.interests)
	myArea, _ := areaByID(p.AreaID)
	var people, mutual []tonightPerson
	for id, other := range s.data {
		oe := s.tn.active(id, now)
		if oe == nil || !s.ready(other) || other.Tier < s.MinDiscoveryTier || !s.eligible(me, id) {
			continue
		}
		oa, ok := areaByID(other.AreaID)
		if !ok {
			continue
		}
		km, err := profile.CellDistanceKm(myArea.Cell, oa.Cell)
		// Within both members' chosen radius (cell centres; the label stays approximate).
		if err != nil || km > float64(e.radiusKm) || km > float64(oe.radiusKm) {
			continue
		}
		label, _ := profile.DistanceBand(myArea.Cell, oa.Cell)
		shown, blurred := s.visiblePhotos(other)
		tp := tonightPerson{Handle: s.tn.handle(me, id), DisplayName: other.DisplayName, Age: s.ageOf(other),
			Area: oa.Label(other.CityOnly), Distance: label, Photos: len(shown), Blurred: blurred,
			Interested: e.interests[id], Mutual: e.interests[id] && oe.interests[me], km: km}
		if tp.Mutual {
			tp.ThreadID = s.ensureThread(context.Background(), me, id, "tonight").ID
			mutual = append(mutual, tp)
		} else {
			people = append(people, tp)
		}
	}
	sort.Slice(people, func(i, j int) bool { return people[i].km < people[j].km })
	if len(people) > maxTonightPeople {
		people = people[:maxTonightPeople]
	}
	if people != nil {
		out["people"] = people
	}
	if mutual != nil {
		out["mutual"] = mutual
	}
	return out
}

func (s *Service) handleTonight(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	writeJSON(w, 200, s.tonightView(acc.ID, p))
}

func (s *Service) handleTonightStart(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		Hours    int `json:"hours"`
		RadiusKm int `json:"radius_km"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in); err != nil ||
		in.Hours < 1 || in.Hours > MaxTonightHours || !allowedRadius[in.RadiusKm] {
		fail(w, 422, "validation_failed", "Choose 1 to 6 hours and a distance.")
		return
	}
	if p.Tier < s.MinDiscoveryTier {
		fail(w, 403, "tier_required", "Finish your ID check to use Tonight.")
		return
	}
	if !s.ready(p) {
		fail(w, 409, "profile_incomplete", "Finish your profile first: a main photo, a name and your area.")
		return
	}
	e := s.tn.active(acc.ID, s.now())
	if e == nil {
		e = &tonightEntry{interests: map[string]bool{}}
		s.tn.entries[acc.ID] = e
	}
	e.until, e.radiusKm = s.now().Add(time.Duration(in.Hours)*time.Hour), in.RadiusKm
	writeJSON(w, 200, s.tonightView(acc.ID, p))
}

func (s *Service) handleTonightStop(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	delete(s.tn.entries, acc.ID)
	writeJSON(w, 200, s.tonightView(acc.ID, p))
}

// resolve maps a viewer's handle to a member who is visible to them right now.
func (s *Service) resolve(viewer, handle string) (string, *memberProfile, bool) {
	id, ok := s.tn.handles[viewer][handle]
	if !ok {
		return "", nil, false
	}
	now := s.now()
	other := s.data[id]
	if other == nil || s.tn.active(viewer, now) == nil || s.tn.active(id, now) == nil || !s.ready(other) || !s.eligible(viewer, id) {
		return "", nil, false
	}
	return id, other, true
}

func (s *Service) handleTonightInterest(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	e := s.tn.active(acc.ID, s.now())
	if e == nil {
		fail(w, 409, "tonight_off", "Switch on Tonight first.")
		return
	}
	id, _, ok := s.resolve(acc.ID, r.PathValue("handle"))
	if !ok {
		fail(w, 404, "not_found", "This member is no longer available tonight.")
		return
	}
	if !e.interests[id] && len(e.interests) >= MaxTonightInterests {
		fail(w, 429, "interest_limit", "You've reached tonight's limit of 10. Wait for a reply or try again tomorrow.")
		return
	}
	e.interests[id] = true
	writeJSON(w, 200, s.tonightView(acc.ID, p))
}

// handleTonightPhoto serves the n-th photo the viewer may see: the main photo and photos the
// member chose to show. Blurred photos are never served here.
func (s *Service) handleTonightPhoto(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	otherID, other, ok := s.resolve(acc.ID, r.PathValue("handle"))
	n, err := strconv.Atoi(r.PathValue("n"))
	if !ok || err != nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	shown, _ := s.visiblePhotos(other)
	if n < 0 || n >= len(shown) {
		fail(w, 404, "not_found", "Not found")
		return
	}
	data, err := s.photoBytes(r.Context(), otherID, shown[n])
	if err != nil {
		fail(w, 404, "not_found", "Not found")
		return
	}
	w.Header().Set("Content-Type", shown[n].ContentType)
	_, _ = w.Write(data)
}
