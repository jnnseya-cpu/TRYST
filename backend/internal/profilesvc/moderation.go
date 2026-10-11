package profilesvc

import (
	"context"

	"github.com/jnnseya-cpu/tryst/backend/internal/profile"
)

// Moderator runs the vendor checks on an upload: synthetic-image detector, reverse-image
// search, CSAM hash matching, nudity classification and face matching against the
// member's verified face (03 §7.5). Provenance is read by the service itself.
type Moderator interface {
	Check(ctx context.Context, accountID string, img []byte) (profile.Checks, error)
}

// NoModerator is the fail-closed default: without vendors every photo waits for a person.
type NoModerator struct{}

func (NoModerator) Check(context.Context, string, []byte) (profile.Checks, error) {
	return profile.Checks{DetectorRan: false}, nil
}

// DevModerator stands in for the vendors in development and automated tests only: it
// reports one face that matches the member and no synthetic signal. Provenance checks
// (AI labels in the file) still run for real. Forbidden outside dev.
type DevModerator struct{}

func (DevModerator) Check(context.Context, string, []byte) (profile.Checks, error) {
	return profile.Checks{DetectorRan: true, SyntheticScore: 0.02, Faces: []profile.Face{{Match: []float64{0.97}}}}, nil
}
