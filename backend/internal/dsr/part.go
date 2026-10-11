// Package dsr implements members' data rights (docs/spec/02_Shared_Contracts.md §5.2, §7;
// ToS cl. 15; UK GDPR Art. 15, 17, 20; FR-038, FR-118, NFR-01, D-54): a full export of what
// TRYST holds about a member, and account deletion by cryptographic erasure with a signed
// completion proof.
//
// Each service owns its data and implements Part; this package only coordinates.
package dsr

import "context"

// Receipt is what one part did on erasure.
type Receipt struct {
	Part     string   `json:"part"`
	Erased   int      `json:"erased"`             // records and objects destroyed
	Retained []string `json:"retained,omitempty"` // declared exceptions (ToS 15.3, 17.4), stated plainly
}

// Part is one service's share of a member's data.
type Part interface {
	Name() string
	// Export returns everything this part holds about the member, in plain form.
	Export(ctx context.Context, account string) (any, error)
	// Erase destroys it. It must be idempotent.
	Erase(ctx context.Context, account string) (Receipt, error)
}
