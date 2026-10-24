// Package vendors holds the production adapters for third-party checks and alerting
// (03 §6.7, §7.5, §7.7; FR-120; D-56): photo checks (Sightengine for synthetic images and
// nudity, Microsoft PhotoDNA for known-CSAM hashes), and a signed HTTPS gateway for the
// vendors whose APIs are contract-specific (liveness and face match from the identity
// provider, the alarm-receiving centre).
//
// Every adapter fails closed: an error or timeout means "not checked", which the callers
// already treat as "a person must look" (photos) or "no" (liveness).
package vendors

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Gateway calls a vendor endpoint with a JSON body, signed so the vendor (or the thin shim
// TRYST runs in front of a vendor SDK) can verify it came from TRYST:
//
//	X-Tryst-Timestamp: <unix seconds>
//	X-Tryst-Signature: v1=<hex HMAC-SHA256(secret, "<timestamp>.<body>")>
//	Authorization: Bearer <api key>   (if set)
//
// This is the documented contract for vendors integrated through a shim; the per-vendor
// mapping is confirmed with each vendor in P0.
type Gateway struct {
	BaseURL string
	APIKey  string
	Secret  string
	HTTP    *http.Client
	Now     func() time.Time
}

// ErrVendor is any non-2xx answer.
var ErrVendor = errors.New("vendor error")

func (g *Gateway) client() *http.Client {
	if g.HTTP != nil {
		return g.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// Sign returns the signature header value for a body at a time (exported for vendor shims
// and tests).
func Sign(secret string, ts int64, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(strconv.FormatInt(ts, 10) + "."))
	m.Write(body)
	return "v1=" + hex.EncodeToString(m.Sum(nil))
}

// Post sends in and decodes the answer into out.
func (g *Gateway) Post(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(g.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	now := time.Now
	if g.Now != nil {
		now = g.Now
	}
	ts := now().Unix()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tryst-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("X-Tryst-Signature", Sign(g.Secret, ts, body))
	if g.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+g.APIKey)
	}
	resp, err := g.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%w: %s http %d", ErrVendor, path, resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

// Liveness is identity.LivenessProvider over the gateway: the provider compares a fresh,
// randomised-challenge capture with the reference it holds for the account (B2, D-18).
// TRYST sends only the account's pseudonymous reference and the SDK's evidence token.
type Liveness struct {
	G *Gateway
	// MinConfidence below which a match is refused even if the provider says yes.
	MinConfidence float64
}

// Start opens the provider's hosted capture for the account's reference. The provider returns
// its session id (kept by TRYST, never shown to the browser as evidence) and the capture page.
func (l *Liveness) Start(ctx context.Context, identityID, returnURL string) (string, string, error) {
	var out struct {
		SessionID string `json:"session_id"`
		URL       string `json:"url"`
	}
	if err := l.G.Post(ctx, "/liveness/sessions", map[string]string{"reference": identityID, "return_url": returnURL}, &out); err != nil {
		return "", "", err
	}
	if out.SessionID == "" || !strings.HasPrefix(out.URL, "https://") {
		return "", "", ErrVendor
	}
	return out.SessionID, out.URL, nil
}

func (l *Liveness) Verify(ctx context.Context, identityID, evidence string) (bool, error) {
	if evidence == "" {
		return false, nil
	}
	var out struct {
		Match      bool    `json:"match"`
		Confidence float64 `json:"confidence"`
		PADPassed  bool    `json:"pad_passed"` // presentation-attack detection
		Injection  bool    `json:"injection_detected"`
	}
	if err := l.G.Post(ctx, "/liveness/verify", map[string]string{"reference": identityID, "evidence": evidence}, &out); err != nil {
		return false, err
	}
	min := l.MinConfidence
	if min == 0 {
		min = 0.9
	}
	return out.Match && out.PADPassed && !out.Injection && out.Confidence >= min, nil
}
