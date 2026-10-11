package vendors

import (
	"context"
	"crypto/hmac"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
)

// Assurance is identity.AssuranceProvider over the gateway contract (D-56, D-61): TRYST
// asks the provider (or its shim beside the provider SDK) for a hosted check session, and
// the provider reports the result to /v1/assurance/callback, signed with CallbackSecret:
//
//	X-Provider-Timestamp: unix seconds (±5 min)
//	X-Provider-Signature: v1=hex(HMAC-SHA256(secret, "<timestamp>.<body>"))
//
// The result carries is_adult, birth_year_month and the document number; TRYST keeps
// only a one-way code of the number.
type Assurance struct {
	G              *Gateway
	CallbackSecret string
	ProviderName   string
	Now            func() time.Time
}

func (a *Assurance) Name() string {
	if a.ProviderName != "" {
		return a.ProviderName
	}
	return "assurance"
}

func (a *Assurance) Start(ctx context.Context, sess identity.AssuranceSession, returnURL string) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	// The provider sees the session id and a pseudonymous reference, never a name or contact.
	if err := a.G.Post(ctx, "/assurance/sessions", map[string]string{"session_id": sess.ID, "kind": sess.Kind,
		"reference": sess.Account, "return_url": returnURL}, &out); err != nil {
		return "", err
	}
	if out.URL == "" {
		return "", ErrVendor
	}
	return out.URL, nil
}

var errCallback = errors.New("assurance callback not authentic")

func (a *Assurance) VerifyCallback(r *http.Request, body []byte) error {
	if a.CallbackSecret == "" {
		return errCallback
	}
	ts, err := strconv.ParseInt(r.Header.Get("X-Provider-Timestamp"), 10, 64)
	if err != nil {
		return errCallback
	}
	now := time.Now
	if a.Now != nil {
		now = a.Now
	}
	if d := now().Sub(time.Unix(ts, 0)); d > 5*time.Minute || d < -5*time.Minute {
		return errCallback
	}
	if !hmac.Equal([]byte(r.Header.Get("X-Provider-Signature")), []byte(Sign(a.CallbackSecret, ts, body))) {
		return errCallback
	}
	return nil
}

var _ identity.AssuranceProvider = (*Assurance)(nil)
