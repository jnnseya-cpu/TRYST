package growth

import (
	"context"

	"github.com/jnnseya-cpu/tryst/backend/internal/svcauth"
)

// RemoteSignals reads the member platform's aggregate counts over the signed internal API.
// The platform answers this caller on that one path only, and only with weekly counts by
// city that clear the cohort floor (k ≥ 50): never a member, an account or an event.
type RemoteSignals struct{ C *svcauth.Client }

func (r RemoteSignals) Aggregates(ctx context.Context) ([]Observation, error) {
	var out struct {
		Items []Observation `json:"items"`
	}
	err := r.C.Do(ctx, "GET", "/internal/v1/growth/aggregates", nil, &out)
	return out.Items, err
}
