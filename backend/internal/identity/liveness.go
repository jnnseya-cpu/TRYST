package identity

import (
	"context"
	"errors"
)

// ErrLivenessUnavailable means no provider is configured or it is down. Login fails closed:
// new mobile logins are refused; existing sessions are unaffected (03 §6.7).
var ErrLivenessUnavailable = errors.New("liveness_unavailable")

// LivenessProvider performs B2: a randomised-challenge liveness check matched 1:1 to the
// account holder's provider-held reference (02 §3.3, D-18). TRYST receives only the result.
type LivenessProvider interface {
	Verify(ctx context.Context, identityID string, evidence string) (match bool, err error)
}

// DevLiveness approves every check. It exists for local development and automated tests only
// and the service refuses to start with it outside the "dev" environment.
type DevLiveness struct{}

func (DevLiveness) Verify(context.Context, string, string) (bool, error) { return true, nil }

// NoLiveness is the fail-closed default when no provider is configured.
type NoLiveness struct{}

func (NoLiveness) Verify(context.Context, string, string) (bool, error) {
	return false, ErrLivenessUnavailable
}
