package profilesvc

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/dsr"
)

// Name implements dsr.Part.
func (s *Service) Name() string { return "profile" }

// Export returns the member's profile, photos (the stored, metadata-free copies), Tonight
// state, conversations (metadata only: messages are end-to-end encrypted and readable only
// on the member's devices), meets and device public keys.
func (s *Service) Export(ctx context.Context, account string) (any, error) {
	s.mu.Lock()
	p := s.data[account]
	var photos []*photo
	out := map[string]any{}
	if p != nil {
		photos = append(photos, p.Photos...)
		area := ""
		if a, ok := areaByID(p.AreaID); ok {
			area = a.Label(p.CityOnly)
		}
		out["profile"] = map[string]any{"display_name": p.DisplayName, "area": area, "city_only": p.CityOnly, "birth_year_month": p.BirthYM}
		out["discretion"] = map[string]any{"status": s.discretionView(p)["status"], "policy": p.policy(), "history": p.PolicyHistory,
			"partner_shield_entries": len(p.Shield), "partner_shield_note": "Stored only as keyed one-way codes; the contacts can't be shown back."}
	}
	if e := s.tn.entries[account]; e != nil {
		out["tonight"] = map[string]any{"until": e.until, "radius_km": e.radiusKm, "interests": len(e.interests)}
	}
	type threadOut struct {
		ID        string    `json:"id"`
		With      string    `json:"with"`
		Created   time.Time `json:"created"`
		State     string    `json:"state"`
		Messages  int64     `json:"messages"`
		Retention int       `json:"retention_days"`
	}
	threads := []threadOut{}
	for _, t := range s.th.threads {
		if t.has(account) {
			with := ""
			if o := s.data[t.other(account)]; o != nil {
				with = o.DisplayName
			}
			threads = append(threads, threadOut{t.ID, with, t.Created, t.State, t.Seq, t.retention()})
		}
	}
	out["conversations"] = threads
	out["conversations_note"] = "Messages are end-to-end encrypted. TRYST holds only ciphertext it cannot read; your messages are on your devices."
	var meets []map[string]any
	for _, m := range s.mt.meets {
		if t := s.th.threads[m.Thread]; t != nil && t.has(account) {
			meets = append(meets, map[string]any{"id": m.ID, "start": m.Start, "end": m.End, "state": m.State, "proposed_by_you": m.Proposer == account})
		}
	}
	out["meets"] = meets
	out["meets_note"] = "Meet places are never sent to TRYST."
	var devs []map[string]any
	for _, d := range s.th.devices[account] {
		devs = append(devs, map[string]any{"id": d.ID, "added": d.Added, "public_key": d.JWK})
	}
	out["devices"] = devs
	out["blocked_members"] = len(s.th.blocked[account])
	if st := s.standing[account]; st != "" {
		out["standing"] = st
	}
	if p != nil {
		out["intake"] = map[string]any{"current": p.Vector, "history": p.VectorHistory}
	}
	var sent, received []map[string]any
	for _, i := range s.in.intents {
		if i.From == account {
			sent = append(sent, map[string]any{"at": i.At, "state": senderState(i), "keys": i.Keys, "via": i.Via})
		} else if i.To == account && i.State != "queued" {
			received = append(received, map[string]any{"at": i.At, "state": i.State})
		}
	}
	out["intents_sent"], out["intents_received"] = sent, received
	if c := s.coupleOf(account); c != nil {
		out["couple"] = map[string]any{"since": c.CosignedAt, "veto_mode": c.VetoMode}
	}
	s.mu.Unlock()

	var ph []map[string]any
	for _, x := range photos {
		item := map[string]any{"id": x.ID, "main": x.Main, "added": x.Added, "review": x.Outcome, "content_type": x.ContentType}
		if b, err := s.photoBytes(ctx, account, x); err == nil {
			item["data_base64"] = base64.StdEncoding.EncodeToString(b)
		}
		ph = append(ph, item)
	}
	out["photos"] = ph
	return out, nil
}

// Erase destroys the member's data key first (every sealed photo, in primaries and backups,
// becomes unreadable at that moment), then removes them from discovery and purges records.
// A permanent removal's standing is kept as the declared exception (ToS 15.3).
func (s *Service) Erase(ctx context.Context, account string) (dsr.Receipt, error) {
	rc := dsr.Receipt{Part: "profile"}
	if err := s.vault.Destroy(ctx, account); err != nil {
		return rc, err
	}
	rc.Erased++

	s.mu.Lock()
	// Out of discovery immediately.
	delete(s.tn.entries, account)
	delete(s.tn.handles, account)
	for _, e := range s.tn.entries {
		delete(e.interests, account)
	}
	var blobs []string
	if p := s.data[account]; p != nil {
		for _, ph := range p.Photos {
			blobs = append(blobs, blobID(account, ph.ID))
		}
		delete(s.data, account)
		rc.Erased++
	}
	delete(s.dirty, account)
	var threads []string
	for id, t := range s.th.threads {
		if t.has(account) {
			threads = append(threads, id)
			delete(s.th.threads, id)
		}
	}
	var meets []string
	for id, m := range s.mt.meets {
		for _, t := range threads {
			if m.Thread == t {
				meets = append(meets, id)
				delete(s.mt.meets, id)
			}
		}
	}
	var plans []string
	for k, ss := range s.mt.sessions {
		if ss.Member == account || containsStr(meets, ss.Meet) {
			delete(s.mt.sessions, k) // trusted contacts go with them
			plans = append(plans, k)
		}
	}
	delete(s.th.devices, account)
	delete(s.th.blocked, account)
	// Introductions: intents, handshakes, couple links and slate marks involving them.
	var intents, handshakes, couples []string
	for id, i := range s.in.intents {
		if i.From == account || i.To == account {
			intents = append(intents, id)
			delete(s.in.intents, id)
		}
	}
	for id, h := range s.in.handshakes {
		if h.side(account) >= 0 {
			handshakes = append(handshakes, id)
			delete(s.in.handshakes, id)
		}
	}
	for id, c := range s.in.couples {
		if c.ProfileA == account || c.ProfileB == account {
			couples = append(couples, id)
			delete(s.in.couples, id)
		}
	}
	for k, inv := range s.in.invites {
		if inv.Inviter == account {
			delete(s.in.invites, k)
		}
	}
	delete(s.in.passed, account)
	delete(s.in.saved, account)
	keepStanding := s.standing[account] == "permanent"
	if !keepStanding {
		delete(s.standing, account)
	}
	s.mu.Unlock()

	del := func(kind, id string) {
		if s.st.Delete(ctx, kind, id) == nil {
			rc.Erased++
		}
	}
	for _, b := range blobs {
		if s.st.DeleteBlob(ctx, b) == nil {
			rc.Erased++
		}
	}
	del("profile", account)
	del("devicekeys", account)
	del("blocks", account)
	for _, t := range threads {
		for _, kind := range []string{"msg:" + t, "msg:" + t + ":client"} {
			docs, _ := s.st.All(ctx, kind)
			for id := range docs {
				del(kind, id)
			}
		}
		del("thread", t)
	}
	for _, m := range meets {
		del("meet", m)
	}
	for _, k := range plans {
		del("safety", k)
	}
	for _, id := range intents {
		del("intent", id)
	}
	for _, id := range handshakes {
		del("handshake", id)
	}
	for _, id := range couples {
		del("couple", id)
	}
	del("slate_marks", account)
	if keepStanding {
		rc.Retained = append(rc.Retained, "Permanent-removal standing (ToS 15.3): kept so the person can't rejoin; no profile, photos or messages are kept with it")
	} else {
		del("standing", account)
	}
	return rc, nil
}

func containsStr(xs []string, x string) bool {
	for _, y := range xs {
		if strings.EqualFold(y, x) {
			return true
		}
	}
	return false
}

var _ dsr.Part = (*Service)(nil)
