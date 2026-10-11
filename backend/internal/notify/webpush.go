package notify

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebPush sends content-free Web Push messages (RFC 8030) authenticated with VAPID
// (RFC 8292). There is no payload at all: the device shows the member's own neutral wording
// from local storage, so nothing about TRYST activity passes through Apple, Google or
// Mozilla push services (FR-026, 03 §5 A1).
type WebPush struct {
	Key     *ecdsa.PrivateKey
	Subject string // "mailto:" or https URL for the push service to contact the operator
	HTTP    *http.Client
	Now     func() time.Time
	// AllowHost reports whether a subscription endpoint's host is a push service. Endpoints
	// come from browsers, so anything else is refused (no server-side request forgery).
	AllowHost func(host string) bool
}

// ErrGone means the subscription no longer exists and should be removed.
var ErrGone = errors.New("push subscription gone")

// ErrEndpoint means the endpoint isn't an allowed push service.
var ErrEndpoint = errors.New("push endpoint not allowed")

// DefaultPushHosts are the browser push services (exact hosts or parent domains).
var DefaultPushHosts = []string{"fcm.googleapis.com", "updates.push.services.mozilla.com", "push.apple.com", "notify.windows.com"}

// HostAllowed matches exact hosts and subdomains of the given list.
func HostAllowed(list []string) func(string) bool {
	return func(h string) bool {
		h = strings.ToLower(h)
		for _, a := range list {
			if h == a || strings.HasSuffix(h, "."+a) {
				return true
			}
		}
		return false
	}
}

// GenerateVAPIDKey makes a fresh P-256 key.
func GenerateVAPIDKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// ParseVAPIDKey reads a base64url-encoded 32-byte private scalar.
func ParseVAPIDKey(s string) (*ecdsa.PrivateKey, error) {
	d, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	if err != nil || len(d) != 32 {
		return nil, errors.New("VAPID key must be a base64url 32-byte P-256 scalar")
	}
	return ecdsa.ParseRawPrivateKey(elliptic.P256(), d)
}

// PublicKey is the application server key browsers subscribe with (base64url, uncompressed).
func (p *WebPush) PublicKey() string {
	b, _ := p.Key.PublicKey.Bytes() // uncompressed point: the wire format
	return base64.RawURLEncoding.EncodeToString(b)
}

func (p *WebPush) jwt(aud string) (string, error) {
	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	enc := base64.RawURLEncoding
	h := enc.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	c, _ := json.Marshal(map[string]any{"aud": aud, "exp": now().Add(12 * time.Hour).Unix(), "sub": p.Subject})
	signing := h + "." + enc.EncodeToString(c)
	digest := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, p.Key, digest[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signing + "." + enc.EncodeToString(sig), nil
}

// ValidEndpoint checks an endpoint before it is stored.
func (p *WebPush) ValidEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	return err == nil && u.Scheme == "https" && u.User == nil && p.AllowHost != nil && p.AllowHost(u.Hostname()) && len(endpoint) <= 1024
}

// Send delivers one empty push. Urgency is normal and messages expire after an hour.
func (p *WebPush) Send(ctx context.Context, endpoint string) error {
	if !p.ValidEndpoint(endpoint) {
		return ErrEndpoint
	}
	u, _ := url.Parse(endpoint)
	tok, err := p.jwt(u.Scheme + "://" + u.Host)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("TTL", "3600")
	req.Header.Set("Urgency", "normal")
	req.Header.Set("Authorization", "vapid t="+tok+", k="+p.PublicKey())
	c := p.HTTP
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	switch {
	case resp.StatusCode == 404 || resp.StatusCode == 410:
		return ErrGone
	case resp.StatusCode/100 != 2:
		return fmt.Errorf("push service: http %d", resp.StatusCode)
	}
	return nil
}
