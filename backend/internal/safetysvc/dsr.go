package safetysvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/dsr"
)

// Name implements dsr.Part.
func (s *Service) Name() string { return "safety" }

// Export returns the member's own reports (what and when, not the other person's details)
// and every decision about them with its reason and appeal.
func (s *Service) Export(_ context.Context, account string) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	type rep struct {
		Reference string    `json:"reference"`
		Category  string    `json:"category"`
		At        time.Time `json:"at"`
		Shared    []string  `json:"messages_you_shared,omitempty"`
		Note      string    `json:"note,omitempty"`
	}
	reports := []rep{}
	for _, c := range s.cases {
		if c.Reporter == account && c.Source == "report" {
			reports = append(reports, rep{c.ID, c.Category, c.Opened, c.Disclosed, c.Note})
		}
	}
	type dec struct {
		Action string    `json:"action"`
		Reason string    `json:"reason"`
		At     time.Time `json:"at"`
		Appeal *Appeal   `json:"appeal,omitempty"`
	}
	decisions := []dec{}
	for _, c := range s.decisionsAbout(account) {
		decisions = append(decisions, dec{c.Decision.Action, c.Decision.Reason, c.Decision.At, c.Appeal})
	}
	return map[string]any{"reports_you_made": reports, "decisions_about_you": decisions, "standing": s.standingOf(account)}, nil
}

// Erase removes the member from every report they made (the case stays for the safety of
// the person reported about, without the reporter) and deletes closed, lower-level cases
// about them. Two things are kept, as the declared exceptions: open cases about them (so
// deleting an account can't destroy evidence under review, ToS 17.4) and permanent removals
// (ToS 15.3).
func (s *Service) Erase(ctx context.Context, account string) (dsr.Receipt, error) {
	rc := dsr.Receipt{Part: "safety"}
	s.mu.Lock()
	defer s.mu.Unlock()
	open, permanent := 0, 0
	for id, c := range s.cases {
		switch {
		case c.Reporter == account && c.Subject != account:
			// A placeholder keeps the count of independent reporters right for the case
			// without linking to anyone: it differs per subject and can't be reversed.
			h := sha256.Sum256([]byte("tryst/erased-reporter/v1\x00" + account + "\x00" + c.Subject))
			c.Reporter, c.Disclosed, c.Note = "erased:"+hex.EncodeToString(h[:8]), nil, ""
			s.save(ctx, c)
			rc.Erased++
		case c.Subject == account && c.State != "closed":
			open++
		case c.Subject == account && c.Decision != nil && c.Decision.Action == "permanent" &&
			(c.Appeal == nil || c.Appeal.Outcome != "overturned"):
			permanent++
		case c.Subject == account:
			delete(s.cases, id)
			if err := s.st.Delete(ctx, "case", id); err != nil {
				return rc, err
			}
			rc.Erased++
		}
	}
	if open > 0 {
		rc.Retained = append(rc.Retained, "Safety reports still under review are kept until decided (ToS 17.4)")
	}
	if permanent > 0 {
		rc.Retained = append(rc.Retained, "A permanent removal record is kept so the person can't rejoin (ToS 15.3)")
	}
	s.audit(ctx, "system", "erasure", "", "member erased")
	return rc, nil
}

var _ dsr.Part = (*Service)(nil)
