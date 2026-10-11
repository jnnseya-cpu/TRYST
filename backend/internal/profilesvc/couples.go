package profilesvc

// Couples (02 §5.4; FR-006, FR-096, FR-128; D-40, D-45, D-65). Two verified members link
// from two different devices: one invites, the other co-signs within 24 hours, approving
// the exact version of the inviter's intake they were shown. Either partner can dissolve
// the link at once, without giving a reason. Changing the veto mode needs both. Every 90
// days both reconfirm; after 14 days without that the couple leaves discovery.
//
// Joining alone stays the default and needs nothing from a partner (D-45): a couple link is
// only for the optional joint presence.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/couple"
)

const (
	reconfirmEvery = 90 * 24 * time.Hour
	reconfirmGrace = 14 * 24 * time.Hour
)

type coupleLink struct {
	ID          string               `json:"id"`
	couple.Link                      // partners, device hashes, approved version, veto mode
	PendingVeto map[string]string    `json:"pending_veto,omitempty"` // account -> proposed mode
	Confirmed   map[string]time.Time `json:"confirmed"`              // last reconfirmation each
	Dissolved   bool                 `json:"dissolved,omitempty"`
}

func (c *coupleLink) partner(account string) string {
	if c.ProfileA == account {
		return c.ProfileB
	}
	return c.ProfileA
}

// lapsed: a partner hasn't reconfirmed within 90 days plus the 14-day grace.
func (c *coupleLink) lapsed(now time.Time) bool {
	for _, a := range []string{c.ProfileA, c.ProfileB} {
		if now.Sub(c.Confirmed[a]) > reconfirmEvery+reconfirmGrace {
			return true
		}
	}
	return false
}

type coupleInvite struct {
	couple.Invite
	Inviter string
}

// coupleOf returns the member's live couple link. Caller holds the lock.
func (s *Service) coupleOf(account string) *coupleLink {
	for _, c := range s.in.couples {
		if !c.Dissolved && (c.ProfileA == account || c.ProfileB == account) {
			return c
		}
	}
	return nil
}

func deviceHash(id string) string {
	h := sha256.Sum256([]byte("device|" + id))
	return hex.EncodeToString(h[:])
}

func (s *Service) coupleView(account string) map[string]any {
	c := s.coupleOf(account)
	if c == nil {
		return map[string]any{"couple": nil}
	}
	name := ""
	if p := s.data[c.partner(account)]; p != nil {
		name = p.DisplayName
	}
	now := s.now()
	due := c.Confirmed[account].Add(reconfirmEvery)
	return map[string]any{"couple": map[string]any{
		"id": c.ID, "partner": name, "veto_mode": c.VetoMode, "since": c.CosignedAt,
		"pending_veto_mode": c.PendingVeto[c.partner(account)], "your_pending_veto_mode": c.PendingVeto[account],
		"reconfirm_due": due, "reconfirm_needed": now.After(due), "in_discovery": !c.lapsed(now),
	}}
}

func (s *Service) handleCouple(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	writeJSON(w, 200, s.coupleView(acc.ID))
}

func (s *Service) handleCoupleInvite(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		DeviceID string `json:"device_id"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in)
	switch {
	case len(in.DeviceID) < 8:
		fail(w, 422, "validation_failed", "This device isn't set up yet. Open Messages once, then try again.")
		return
	case p.Tier < "V2":
		fail(w, 403, "tier_required", "Both partners must have finished the ID check.")
		return
	case s.coupleOf(acc.ID) != nil:
		fail(w, 409, "already_linked", "You're already linked.")
		return
	case p.Vector == nil:
		fail(w, 409, "intake_required", "Tell us what you're looking for first; your partner approves it.")
		return
	}
	tok := introID("ci_")
	h := sha256.Sum256([]byte(tok))
	s.in.invites[hex.EncodeToString(h[:])] = &coupleInvite{Inviter: acc.ID, Invite: couple.Invite{
		InviterProfile: acc.ID, InviterDeviceHash: deviceHash(in.DeviceID), InviterTierV2: true,
		ProfileVersion: p.Vector.Version, CreatedAt: s.now()}}
	writeJSON(w, 201, map[string]any{"token": tok, "expires_at": s.now().Add(couple.InviteTTL).UTC().Format(time.RFC3339)})
}

func (s *Service) inviteFor(tok string) *coupleInvite {
	h := sha256.Sum256([]byte(tok))
	return s.in.invites[hex.EncodeToString(h[:])]
}

// handleInvitePreview shows the invitee what they are approving (the exact version).
func (s *Service) handleInvitePreview(w http.ResponseWriter, r *http.Request) {
	_, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	inv := s.inviteFor(r.URL.Query().Get("token"))
	if inv == nil || inv.ConsumedAt != nil || s.now().Sub(inv.CreatedAt) > couple.InviteTTL {
		fail(w, 404, "not_found", "This invitation has expired.")
		return
	}
	p := s.data[inv.Inviter]
	if p == nil || p.Vector == nil {
		fail(w, 404, "not_found", "This invitation has expired.")
		return
	}
	writeJSON(w, 200, map[string]any{"inviter": p.DisplayName, "version": inv.ProfileVersion, "vector": p.Vector})
}

func (s *Service) handleCosign(w http.ResponseWriter, r *http.Request) {
	acc, p, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		Token           string `json:"token"`
		DeviceID        string `json:"device_id"`
		ApprovedVersion int    `json:"approved_version"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in)
	inv := s.inviteFor(in.Token)
	if inv == nil || len(in.DeviceID) < 8 {
		fail(w, 404, "not_found", "This invitation has expired.")
		return
	}
	if s.coupleOf(acc.ID) != nil || s.coupleOf(inv.Inviter) != nil {
		fail(w, 409, "already_linked", "One of you is already linked.")
		return
	}
	link, err := couple.Cosign(&inv.Invite, acc.ID, deviceHash(in.DeviceID), p.Tier >= "V2", in.ApprovedVersion, s.now())
	switch {
	case errors.Is(err, couple.ErrSameDevice):
		fail(w, 409, "same_device_cosign", "Your partner must confirm on their own device.")
		return
	case errors.Is(err, couple.ErrVersionMismatch):
		fail(w, 409, "version_changed", "Your partner changed their answers. Look again, then confirm.")
		return
	case errors.Is(err, couple.ErrTierRequired):
		fail(w, 403, "tier_required", "Both partners must have finished the ID check.")
		return
	case err != nil:
		fail(w, 404, "not_found", "This invitation has expired.")
		return
	}
	now := s.now()
	c := &coupleLink{ID: introID("cp_"), Link: link, Confirmed: map[string]time.Time{link.ProfileA: now, link.ProfileB: now}}
	s.in.couples[c.ID] = c
	s.put(r.Context(), "couple", c.ID, c)
	s.notify(inv.Inviter, "couple", "/couple")
	writeJSON(w, 201, s.coupleView(acc.ID))
}

func (s *Service) ownCouple(w http.ResponseWriter, account, id string) *coupleLink {
	c := s.coupleOf(account)
	if c == nil || c.ID != id {
		fail(w, 404, "not_found", "Not found")
		return nil
	}
	return c
}

// handleVetoMode: a change takes effect only when both partners ask for the same mode.
func (s *Service) handleVetoMode(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	var in struct {
		Mode string `json:"veto_mode"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, maxJSON)).Decode(&in)
	c := s.ownCouple(w, acc.ID, r.PathValue("id"))
	if c == nil {
		return
	}
	if in.Mode != "either" && in.Mode != "both" {
		fail(w, 422, "validation_failed", "Choose either or both.")
		return
	}
	if c.PendingVeto == nil {
		c.PendingVeto = map[string]string{}
	}
	c.PendingVeto[acc.ID] = in.Mode
	if c.PendingVeto[c.partner(acc.ID)] == in.Mode {
		c.VetoMode, c.PendingVeto = in.Mode, nil
	} else {
		s.notify(c.partner(acc.ID), "couple", "/couple")
	}
	s.put(r.Context(), "couple", c.ID, c)
	writeJSON(w, 200, s.coupleView(acc.ID))
}

func (s *Service) handleReconfirm(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	c := s.ownCouple(w, acc.ID, r.PathValue("id"))
	if c == nil {
		return
	}
	c.Confirmed[acc.ID] = s.now()
	s.put(r.Context(), "couple", c.ID, c)
	writeJSON(w, 200, s.coupleView(acc.ID))
}

// handleDissolve ends the link at once. The partner is told only that it ended.
func (s *Service) handleDissolve(w http.ResponseWriter, r *http.Request) {
	acc, _, ok := s.member(w, r)
	if !ok {
		return
	}
	defer s.mu.Unlock()
	c := s.ownCouple(w, acc.ID, r.PathValue("id"))
	if c == nil {
		return
	}
	c.Dissolved = true
	s.put(r.Context(), "couple", c.ID, c)
	s.notify(c.partner(acc.ID), "couple", "/couple")
	writeJSON(w, 200, s.coupleView(acc.ID))
}
