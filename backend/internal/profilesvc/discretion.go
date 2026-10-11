package profilesvc

// Discretion (02 §5.2, §5.3, §6.4; FR-005, FR-011, FR-046, FR-106, FR-127; D-45, D-64).
//
//   - Pause takes the member out of discovery; vanish also closes every conversation. The
//     account and profile are kept. Coming back needs step-up.
//   - The Discretion Policy is versioned. Any change that shows the member to more people
//     (visible instead of incognito, a longer distance, a lower counterpart tier, fewer
//     excluded areas, a smaller partner shield) needs a fresh step-up; tightening never does.
//   - Excluded areas hide the member from people in those areas and hide those people from
//     the member, both ways and silently (FR-011).
//   - Partner shield: keyed hashes of contacts; if one belongs to a member, the two are
//     invisible to each other, both ways and silently (FR-106).
//   - Emergency lock (identity) hides the profile and its photos until recovery.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"time"
)

// Member status (02 §4.2 profile.status).
const (
	statusActive   = ""
	statusPaused   = "paused"
	statusVanished = "vanished"
	statusLocked   = "locked"
)

const (
	maxZones      = 4
	maxShield     = 10
	policyHistory = 50
)

var distances = []int{5, 10, 25, 50}

// Policy is one version of a member's Discretion Policy (the parts discovery uses today).
type Policy struct {
	Version             int       `json:"version"`
	Visibility          string    `json:"visibility"`            // visible | incognito
	MaxDistanceKm       int       `json:"max_distance_km"`       // 5 | 10 | 25 | 50
	MinCounterpartyTier string    `json:"min_counterparty_tier"` // V2 | V3
	Zones               []string  `json:"zones"`                 // excluded area IDs
	At                  time.Time `json:"at"`
}

func defaultPolicy() Policy {
	return Policy{Visibility: "visible", MaxDistanceKm: 50, MinCounterpartyTier: "V2", Zones: []string{}}
}

func (p *memberProfile) policy() Policy {
	if p.Policy == nil {
		return defaultPolicy()
	}
	return *p.Policy
}

// broadens reports whether next shows the member to anyone old didn't.
func broadens(old, next Policy) bool {
	if old.Visibility == "incognito" && next.Visibility != "incognito" {
		return true
	}
	if next.MaxDistanceKm > old.MaxDistanceKm || next.MinCounterpartyTier < old.MinCounterpartyTier {
		return true
	}
	for _, z := range old.Zones {
		if !slices.Contains(next.Zones, z) {
			return true
		}
	}
	return false
}

// hidden reports a member who must not appear anywhere (paused, vanished or locked).
func hidden(p *memberProfile) bool { return p == nil || p.Status != statusActive }

func (s *Service) areaCell(p *memberProfile) string {
	a, _ := areaByID(p.AreaID)
	return a.Cell
}

// discreet applies both members' discretion to a pair. Caller holds the lock.
func (s *Service) discreet(viewer, member string) bool {
	v, m := s.data[viewer], s.data[member]
	if hidden(v) || hidden(m) {
		return false
	}
	vp, mp := v.policy(), m.policy()
	if m.Tier < vp.MinCounterpartyTier || v.Tier < mp.MinCounterpartyTier {
		return false
	}
	vc, mc := s.areaCell(v), s.areaCell(m)
	for _, z := range vp.Zones {
		if a, ok := areaByID(z); ok && a.Cell == mc {
			return false
		}
	}
	for _, z := range mp.Zones {
		if a, ok := areaByID(z); ok && a.Cell == vc {
			return false
		}
	}
	if s.ShieldKeyOf != nil {
		if k := s.ShieldKeyOf(member); k != "" && slices.Contains(v.Shield, k) {
			return false
		}
		if k := s.ShieldKeyOf(viewer); k != "" && slices.Contains(m.Shield, k) {
			return false
		}
	}
	return true
}

// incognitoOK: an incognito member is shown only to people they've shown interest in.
func (s *Service) incognitoOK(viewer, member string) bool {
	m := s.data[member]
	if m == nil || m.policy().Visibility != "incognito" {
		return true
	}
	e := s.tn.entries[member]
	return (e != nil && e.interests[viewer]) || s.sentTo(member, viewer) != nil
}

func (s *Service) discretionView(p *memberProfile) map[string]any {
	status := p.Status
	if status == statusActive {
		status = "active"
	}
	return map[string]any{"status": status, "policy": p.policy(), "versions": len(p.PolicyHistory), "shield": len(p.Shield)}
}

func (s *Service) handleDiscretion(w http.ResponseWriter, r *http.Request) {
	_, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	writeJSON(w, 200, s.discretionView(p))
}

func stepUpRequired(w http.ResponseWriter, what string) {
	fail(w, 403, "step_up_required", "Confirm it's you to "+what+".")
}

// setPolicy records a new version. Caller holds the lock.
func (s *Service) setPolicy(p *memberProfile, next Policy) {
	old := p.policy()
	next.Version, next.At = old.Version+1, s.now().UTC()
	if next.Zones == nil {
		next.Zones = []string{}
	}
	p.Policy = &next
	p.PolicyHistory = append(p.PolicyHistory, next)
	if len(p.PolicyHistory) > policyHistory {
		p.PolicyHistory = p.PolicyHistory[len(p.PolicyHistory)-policyHistory:]
	}
}

func (s *Service) handlePolicy(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		Visibility          string `json:"visibility"`
		MaxDistanceKm       int    `json:"max_distance_km"`
		MinCounterpartyTier string `json:"min_counterparty_tier"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSON))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || (in.Visibility != "visible" && in.Visibility != "incognito") ||
		!slices.Contains(distances, in.MaxDistanceKm) || (in.MinCounterpartyTier != "V2" && in.MinCounterpartyTier != "V3") {
		fail(w, 422, "validation_failed", "Choose visibility, a distance and who can see you.")
		return
	}
	old := p.policy()
	next := old
	next.Visibility, next.MaxDistanceKm, next.MinCounterpartyTier = in.Visibility, in.MaxDistanceKm, in.MinCounterpartyTier
	if broadens(old, next) && !acc.StepUp {
		stepUpRequired(w, "show your profile to more people")
		return
	}
	s.setPolicy(p, next)
	if e := s.tn.entries[acc.ID]; e != nil && e.radiusKm > next.MaxDistanceKm {
		e.radiusKm = next.MaxDistanceKm
	}
	writeJSON(w, 200, s.discretionView(p))
}

func (s *Service) handleZones(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		Zones []string `json:"zones"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSON))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || len(in.Zones) > maxZones {
		fail(w, 422, "validation_failed", "Choose up to 4 areas.")
		return
	}
	zones := []string{}
	for _, z := range in.Zones {
		if _, ok := areaByID(z); !ok {
			fail(w, 422, "validation_failed", "Choose areas from the list.")
			return
		}
		if !slices.Contains(zones, z) {
			zones = append(zones, z)
		}
	}
	old := p.policy()
	next := old
	next.Zones = zones
	if broadens(old, next) && !acc.StepUp {
		stepUpRequired(w, "remove a hidden area")
		return
	}
	s.setPolicy(p, next)
	writeJSON(w, 200, s.discretionView(p))
}

// handleShieldAdd adds device-hashed contacts to the partner shield.
func (s *Service) handleShieldAdd(w http.ResponseWriter, r *http.Request) {
	_, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		Add []string `json:"add"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSON))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || len(in.Add) == 0 || len(p.Shield)+len(in.Add) > maxShield || s.BlindShield == nil {
		fail(w, 422, "validation_failed", "Add up to 10 contacts.")
		return
	}
	for _, d := range in.Add {
		k := s.BlindShield(d)
		if k == "" {
			fail(w, 422, "validation_failed", "That contact couldn't be added.")
			return
		}
		if !slices.Contains(p.Shield, k) {
			p.Shield = append(p.Shield, k)
		}
	}
	writeJSON(w, 200, s.discretionView(p))
}

// handleShieldClear empties the shield (broadening: step-up).
func (s *Service) handleShieldClear(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	if len(p.Shield) > 0 && !acc.StepUp {
		stepUpRequired(w, "remove your partner shield")
		return
	}
	p.Shield = nil
	writeJSON(w, 200, s.discretionView(p))
}

func (s *Service) handlePause(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	if p.Status == statusActive {
		p.Status = statusPaused
	}
	delete(s.tn.entries, acc.ID)
	writeJSON(w, 200, s.discretionView(p))
}

// handleVanish hides the member and closes every conversation; the other member sees it
// closed, never why.
func (s *Service) handleVanish(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	if p.Status != statusLocked {
		p.Status = statusVanished
	}
	delete(s.tn.entries, acc.ID)
	for _, t := range s.th.threads {
		if t.has(acc.ID) && t.State == "normal" {
			t.State = "closed"
			s.saveThread(r.Context(), t)
		}
	}
	writeJSON(w, 200, s.discretionView(p))
}

func (s *Service) handleResume(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	if p.Status == statusActive {
		writeJSON(w, 200, s.discretionView(p))
		return
	}
	if !acc.StepUp {
		stepUpRequired(w, "come back to discovery")
		return
	}
	p.Status = statusActive
	writeJSON(w, 200, s.discretionView(p))
}

// Lock hides (or, after recovery, pauses) a member's profile (identity's emergency lock).
// After recovery the member comes back paused and resumes when ready.
func (s *Service) Lock(account string, locked bool) {
	s.mu.Lock()
	p := s.data[account]
	if p == nil {
		p = &memberProfile{}
		s.data[account] = p
	}
	if locked {
		p.Status = statusLocked
		delete(s.tn.entries, account)
	} else if p.Status == statusLocked {
		p.Status = statusPaused
	}
	s.dirty[account] = true
	s.mu.Unlock()
	s.flush(context.Background())
}
