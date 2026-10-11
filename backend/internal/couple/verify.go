package couple

import (
	"bytes"
	"errors"
	"fmt"
	"time"
)

// Couple verification (FR-095, FR-096; D-40).
//
// What TRYST can prove about a couple, and does:
//  1. Two different, real, verified adults: each partner passed V2 (ID document + liveness)
//     on their own account and device; their ID documents differ; their faces do not match
//     each other (one person cannot pose as both partners with two accounts).
//  2. Both consent: each partner co-signs from their own device with their own biometric
//     sign-in (Cosign, above), and both approve the exact profile version.
//  3. They are together: one live in-app capture, with a randomised liveness challenge,
//     showing both faces, each matched to that partner's verified face; plus an in-person
//     pairing (one phone scans the other's code) within minutes of the capture.
//  4. It stays true: either partner can dissolve the couple at any moment, and both must
//     reconfirm every 90 days or the couple profile pauses.
//
// What TRYST does not and cannot verify: marriage or the nature of the relationship
// (ToS cl. 5.2). Couples are not asked for certificates; open and unmarried couples are
// equally welcome.

const (
	PartnerFaceMax = 0.50 // partners' verified faces must NOT match each other above this
	JointMatchMin  = 0.90 // each face in the joint capture vs that partner's verified face
	CaptureMaxAge  = 10 * time.Minute
	PairingWindow  = 2 * time.Minute
	ReconfirmEvery = 90 * 24 * time.Hour
	ReconfirmGrace = 14 * 24 * time.Hour
)

// Partner is one partner's verified state.
type Partner struct {
	ProfileID      string
	V2             bool
	Adult          bool
	DocAnchor      []byte // HMAC of the ID document (anchor candidate, 02 §4.1)
	DeviceHash     string
	InActiveCouple bool
}

// JointCapture is the live in-app capture of both partners together.
type JointCapture struct {
	LivenessPassed bool // presentation-attack detection on the capture (no photo of a photo)
	MatchA, MatchB float64
	FaceCount      int
	CapturedAt     time.Time
}

// Pairing is the in-person pairing: partner B's phone scans the code on partner A's phone,
// then both approve with their own biometric sign-in.
type Pairing struct {
	ScannedAt, AApprovedAt, BApprovedAt time.Time
}

// Evidence is everything checked before a couple profile is marked verified.
type Evidence struct {
	A, B             Partner
	PartnerFaceScore float64 // A's verified face vs B's verified face, from the provider
	Capture          JointCapture
	Pairing          Pairing
}

var ErrNotVerified = errors.New("couple not verified")

// Verify returns nil if every check passes, or an error listing every failed check in
// plain words (shown to both partners).
func Verify(e Evidence, now time.Time) error {
	var fails []string
	add := func(cond bool, msg string) {
		if !cond {
			fails = append(fails, msg)
		}
	}
	add(e.A.V2 && e.B.V2, "both partners must complete ID and face verification")
	add(e.A.Adult && e.B.Adult, "both partners must be 18 or over")
	add(e.A.ProfileID != e.B.ProfileID, "two different accounts are needed")
	add(e.A.DeviceHash != e.B.DeviceHash, "each partner must use their own phone")
	add(len(e.A.DocAnchor) > 0 && len(e.B.DocAnchor) > 0 && !bytes.Equal(e.A.DocAnchor, e.B.DocAnchor),
		"each partner must verify with their own ID document")
	add(e.PartnerFaceScore < PartnerFaceMax, "the two verified faces look like the same person")
	add(!e.A.InActiveCouple && !e.B.InActiveCouple, "a partner is already in another couple profile")
	c := e.Capture
	add(c.LivenessPassed, "the photo of you together must be taken live in the app")
	add(c.FaceCount == 2, "the photo of you together must show exactly two faces")
	add(c.MatchA >= JointMatchMin && c.MatchB >= JointMatchMin, "both partners must be clearly recognisable in the photo together")
	add(!c.CapturedAt.IsZero() && now.Sub(c.CapturedAt) <= CaptureMaxAge && !c.CapturedAt.After(now),
		"the photo together must be taken in the last 10 minutes")
	p := e.Pairing
	inWindow := func(t time.Time) bool {
		return !t.IsZero() && absDur(t.Sub(p.ScannedAt)) <= PairingWindow
	}
	add(!p.ScannedAt.IsZero() && inWindow(p.AApprovedAt) && inWindow(p.BApprovedAt),
		"both phones must be paired in person and approved within 2 minutes")
	add(!p.ScannedAt.IsZero() && absDur(p.ScannedAt.Sub(c.CapturedAt)) <= CaptureMaxAge,
		"pairing and the photo together must happen in the same session")
	if len(fails) > 0 {
		return fmt.Errorf("%w: %v", ErrNotVerified, fails)
	}
	return nil
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// State of a verified couple over time.
type State string

const (
	Active       State = "active"
	ReconfirmDue State = "reconfirm_due" // still visible; both partners are asked to confirm
	Paused       State = "paused"        // hidden from discovery until both confirm
	Dissolved    State = "dissolved"
)

// Status applies the 90-day reconfirmation: each partner confirms with their own sign-in.
// Dissolution by either partner is immediate and final for that link.
func Status(lastConfirmA, lastConfirmB time.Time, dissolved bool, now time.Time) State {
	if dissolved {
		return Dissolved
	}
	oldest := lastConfirmA
	if lastConfirmB.Before(oldest) {
		oldest = lastConfirmB
	}
	switch age := now.Sub(oldest); {
	case age <= ReconfirmEvery:
		return Active
	case age <= ReconfirmEvery+ReconfirmGrace:
		return ReconfirmDue
	}
	return Paused
}

// Label is the factual, dated statement shown on a verified couple profile (P6: no
// "safe" badges, only dated facts).
func Label(verifiedAt time.Time) string {
	return "Couple · both partners verified " + verifiedAt.Format("Jan 2006")
}
