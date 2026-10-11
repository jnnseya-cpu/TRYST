package identity

import (
	"context"
	"encoding/hex"

	"github.com/jnnseya-cpu/tryst/backend/internal/admission"

	"github.com/jnnseya-cpu/tryst/backend/internal/dsr"
)

// Name implements dsr.Part.
func (s *Server) Name() string { return "identity" }

// Export returns the account record: verification level, birth year-month from the ID check
// (never the day), passkeys (no key material) and the member's own security activity. The
// contact is held only as a keyed hash, so it can't be exported in clear.
func (s *Server) Export(_ context.Context, account string) (any, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	i, ok := s.store.identities[account]
	if !ok {
		return nil, nil
	}
	var keys []map[string]any
	for _, c := range i.Credentials {
		keys = append(keys, map[string]any{"surface": c.Surface, "kind": c.Kind, "created": c.CreatedAt, "revoked": c.Revoked})
	}
	sessions := 0
	for _, se := range s.store.sessions {
		if se.IdentityID == account {
			sessions++
		}
	}
	return map[string]any{
		"created": i.CreatedAt, "verification": i.Tier.String(), "jurisdiction": i.Jurisdiction, "birth_year_month": i.BirthYM,
		"passkeys": keys, "signed_in_sessions": sessions, "security_activity": i.Events,
		"contact_note": "Your email or phone number is stored only as a one-way code, so we can't show it back to you.",
	}, nil
}

// Erase removes the account, its passkeys and every session. The contact hash goes with it,
// so the same email or phone can be used to join again.
func (s *Server) Erase(ctx context.Context, account string) (dsr.Receipt, error) {
	rc := dsr.Receipt{Part: "identity"}
	s.store.mu.Lock()
	if i, ok := s.store.identities[account]; ok {
		if b, err := hex.DecodeString(i.AnchorDoc); err == nil && len(b) > 0 {
			s.as.index.Remove(account, admission.Anchors{Doc: b}) // a ban, if any, stays (ToS 15.3)
		}
		delete(s.store.byContact, i.ContactHash)
		delete(s.store.identities, account)
		s.store.dirty[account] = true // flush deletes it
		rc.Erased++
	}
	for th, se := range s.store.sessions {
		if se.IdentityID == account {
			delete(s.store.sessions, th)
			s.store.sessDirty[th] = nil
			rc.Erased++
		}
	}
	s.store.mu.Unlock()
	s.flush(ctx)
	if s.cfg.Persist == nil {
		return rc, nil
	}
	// Idempotent: also clear anything a previous, interrupted erasure left behind.
	if err := s.cfg.Persist.Delete(ctx, "identity", account); err != nil {
		return rc, err
	}
	return rc, nil
}

var _ dsr.Part = (*Server)(nil)
