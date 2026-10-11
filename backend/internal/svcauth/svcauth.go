// Package svcauth signs and verifies calls between TRYST services (D-59). Internal APIs
// listen on a private port and also check a signature, so a mis-routed request or a
// compromised neighbour on the network can't call them:
//
//	X-Svc-Caller:    the calling service's name
//	X-Svc-Timestamp: unix seconds (±2 minutes accepted)
//	X-Svc-Signature: v1=hex(HMAC-SHA256(secret, caller "\n" timestamp "\n" method "\n" path-and-query "\n" sha256hex(body)))
//
// Each direction has its own secret. Bodies are small JSON documents.
package svcauth

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

const (
	headerCaller = "X-Svc-Caller"
	headerTime   = "X-Svc-Timestamp"
	headerSig    = "X-Svc-Signature"
	window       = 2 * time.Minute
	maxBody      = 8 << 20
)

// ErrMinSecret is returned for secrets that are too short to be safe.
var ErrMinSecret = errors.New("svcauth: secret must be at least 32 bytes")

func digest(secret []byte, caller, ts, method, target string, body []byte) string {
	bh := sha256.Sum256(body)
	m := hmac.New(sha256.New, secret)
	fmt.Fprintf(m, "%s\n%s\n%s\n%s\n%s", caller, ts, method, target, hex.EncodeToString(bh[:]))
	return "v1=" + hex.EncodeToString(m.Sum(nil))
}

// Client calls another service's internal API.
type Client struct {
	BaseURL string
	Caller  string
	Secret  []byte
	HTTP    *http.Client
	Now     func() time.Time
}

// ErrStatus is a non-2xx answer.
type ErrStatus struct{ Code int }

func (e *ErrStatus) Error() string { return "svcauth: http " + strconv.Itoa(e.Code) }

// IsStatus reports whether err is an ErrStatus with the given code.
func IsStatus(err error, code int) bool {
	var e *ErrStatus
	return errors.As(err, &e) && e.Code == code
}

// Do sends method path with in as JSON (nil for no body) and decodes JSON into out (if not
// nil). For a non-JSON answer use Raw.
func (c *Client) Do(ctx context.Context, method, path string, in, out any) error {
	b, _, err := c.Raw(ctx, method, path, in)
	if err != nil {
		return err
	}
	if out != nil && len(b) > 0 {
		return json.Unmarshal(b, out)
	}
	return nil
}

// Raw sends a request and returns the body and content type.
func (c *Client) Raw(ctx context.Context, method, path string, in any) ([]byte, string, error) {
	var body []byte
	if in != nil {
		var err error
		if body, err = json.Marshal(in); err != nil {
			return nil, "", err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	ts := strconv.FormatInt(now().Unix(), 10)
	req.Header.Set(headerCaller, c.Caller)
	req.Header.Set(headerTime, ts)
	req.Header.Set(headerSig, digest(c.Secret, c.Caller, ts, method, req.URL.RequestURI(), body))
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode/100 != 2 {
		return nil, "", &ErrStatus{resp.StatusCode}
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// Verifier checks incoming internal calls.
type Verifier struct {
	// Secrets by caller name.
	Secrets map[string][]byte
	Now     func() time.Time
}

// Wrap returns a handler that rejects unsigned, mis-signed or stale calls with 401 and
// passes the verified caller in the request header (overwriting anything sent).
func (v *Verifier) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := r.Header.Get(headerCaller)
		secret, ok := v.Secrets[caller]
		ts := r.Header.Get(headerTime)
		n, err := strconv.ParseInt(ts, 10, 64)
		now := time.Now
		if v.Now != nil {
			now = v.Now
		}
		body, rerr := io.ReadAll(io.LimitReader(r.Body, maxBody))
		d := now().Sub(time.Unix(n, 0))
		if !ok || err != nil || rerr != nil || d > window || d < -window ||
			!hmac.Equal([]byte(r.Header.Get(headerSig)), []byte(digest(secret, caller, ts, r.Method, r.URL.RequestURI(), body))) {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "unauthorised", http.StatusUnauthorized)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// Caller returns the verified caller of a request that passed Wrap.
func Caller(r *http.Request) string { return r.Header.Get(headerCaller) }

// Secret reads a hex secret, enforcing the minimum length.
func Secret(hexValue string) ([]byte, error) {
	b, err := hex.DecodeString(strings.TrimSpace(hexValue))
	if err != nil || len(b) < 32 {
		return nil, ErrMinSecret
	}
	return b, nil
}
