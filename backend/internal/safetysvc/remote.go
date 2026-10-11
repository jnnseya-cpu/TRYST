package safetysvc

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/dsr"
	"github.com/jnnseya-cpu/tryst/backend/internal/gate"
	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcauth"
)

// Remote clients (D-59): the standalone safety service reaches the member platform's
// internal API, and the member platform reaches the safety service, through these.

// RemoteMembers authenticates members by asking the identity service about the bearer
// token the member sent. Only the account id, tier and birth year-month come back.
type RemoteMembers struct{ C *svcauth.Client }

func (m RemoteMembers) Authenticate(r *http.Request) (identity.Account, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return identity.Account{}, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var out struct {
		Account string `json:"account"`
		Tier    int    `json:"tier"`
		BirthYM string `json:"birth_ym"`
	}
	if err := m.C.Do(ctx, "POST", "/internal/v1/authenticate", map[string]string{"authorization": h}, &out); err != nil || out.Account == "" {
		return identity.Account{}, false
	}
	return identity.Account{ID: out.Account, Tier: gate.Tier(out.Tier), BirthYM: out.BirthYM}, true
}

// RemoteDirectory is Directory over the member platform's internal API.
type RemoteDirectory struct{ C *svcauth.Client }

func (d RemoteDirectory) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func (d RemoteDirectory) Counterpart(me, threadID string) (string, bool) {
	ctx, cancel := d.ctx()
	defer cancel()
	var out struct {
		Member string `json:"member"`
	}
	err := d.C.Do(ctx, "GET", "/internal/v1/threads/"+url.PathEscape(threadID)+"/counterpart?member="+url.QueryEscape(me), nil, &out)
	return out.Member, err == nil && out.Member != ""
}

func (d RemoteDirectory) Summary(account string) (string, string) {
	ctx, cancel := d.ctx()
	defer cancel()
	var out struct{ Name, Area string }
	_ = d.C.Do(ctx, "GET", "/internal/v1/members/"+url.PathEscape(account)+"/summary", nil, &out)
	return out.Name, out.Area
}

func (d RemoteDirectory) MainPhoto(ctx context.Context, account string) ([]byte, string, error) {
	return d.C.Raw(ctx, "GET", "/internal/v1/members/"+url.PathEscape(account)+"/main-photo", nil)
}

func (d RemoteDirectory) SetStanding(ctx context.Context, account, standing string) error {
	return d.C.Do(ctx, "PUT", "/internal/v1/members/"+url.PathEscape(account)+"/standing", map[string]string{"standing": standing}, nil)
}

// RemoteNotices delivers safety notices to the member platform's notification service.
func RemoteNotices(c *svcauth.Client) func(ctx context.Context, account, kind, link string) error {
	return func(ctx context.Context, account, kind, link string) error {
		return c.Do(ctx, "POST", "/internal/v1/notices", map[string]string{"account": account, "kind": kind, "link": link}, nil)
	}
}

// RemotePart is the safety service's share of data rights, called by the member
// platform's export and erasure coordinator.
type RemotePart struct{ C *svcauth.Client }

func (p RemotePart) Name() string { return "safety" }

func (p RemotePart) Export(ctx context.Context, account string) (any, error) {
	var out map[string]any
	err := p.C.Do(ctx, "POST", "/internal/v1/export", map[string]string{"account": account}, &out)
	return out, err
}

func (p RemotePart) Erase(ctx context.Context, account string) (dsr.Receipt, error) {
	var rc dsr.Receipt
	err := p.C.Do(ctx, "POST", "/internal/v1/erase", map[string]string{"account": account}, &rc)
	return rc, err
}

var _ dsr.Part = RemotePart{}
