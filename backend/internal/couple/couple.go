// Package couple enforces VC co-signature (docs/spec/02_Shared_Contracts.md §3.1, §5.4;
// FR-006, G-NG-5): both partners independently at V2, each confirming from their own
// distinct device, both approving the exact profile version.
package couple

import (
	"errors"
	"time"
)

var (
	ErrInviteExpired   = errors.New("invite expired")
	ErrInviteUsed      = errors.New("invite already used")
	ErrSameDevice      = errors.New("same_device_cosign")
	ErrSameMember      = errors.New("a member cannot co-sign with themselves")
	ErrTierRequired    = errors.New("both partners must be V2")
	ErrVersionMismatch = errors.New("both partners must approve the same profile version")
)

const InviteTTL = 24 * time.Hour

type Invite struct {
	InviterProfile    string
	InviterDeviceHash string
	InviterTierV2     bool
	ProfileVersion    int
	CreatedAt         time.Time
	ConsumedAt        *time.Time
}

type Link struct {
	ProfileA, ProfileB       string
	ADeviceHash, BDeviceHash string
	ApprovedVersion          int
	VetoMode                 string // either | both
	CosignedAt               time.Time
}

// Cosign validates partner B's confirmation and returns the link. It is the only way a
// couple_link row is created.
func Cosign(inv *Invite, profileB, deviceHashB string, bTierV2 bool, approvedVersion int, now time.Time) (Link, error) {
	switch {
	case inv.ConsumedAt != nil:
		return Link{}, ErrInviteUsed
	case now.Sub(inv.CreatedAt) > InviteTTL:
		return Link{}, ErrInviteExpired
	case profileB == inv.InviterProfile:
		return Link{}, ErrSameMember
	case deviceHashB == inv.InviterDeviceHash:
		return Link{}, ErrSameDevice
	case !inv.InviterTierV2 || !bTierV2:
		return Link{}, ErrTierRequired
	case approvedVersion != inv.ProfileVersion:
		return Link{}, ErrVersionMismatch
	}
	inv.ConsumedAt = &now
	return Link{
		ProfileA: inv.InviterProfile, ProfileB: profileB,
		ADeviceHash: inv.InviterDeviceHash, BDeviceHash: deviceHashB,
		ApprovedVersion: approvedVersion, VetoMode: "either", CosignedAt: now,
	}, nil
}

// Decision is one partner's view on a candidate or handshake.
type Decision int

const (
	Pending Decision = iota
	Accept
	Decline
)

// CoupleDecision applies the veto mode (02 §9): with "either", any decline is final and one
// accept suffices only if the other has not declined; with "both", both must accept.
func CoupleDecision(vetoMode string, a, b Decision) Decision {
	if a == Decline || b == Decline {
		return Decline
	}
	if vetoMode == "both" {
		if a == Accept && b == Accept {
			return Accept
		}
		return Pending
	}
	if a == Accept || b == Accept {
		return Accept
	}
	return Pending
}
