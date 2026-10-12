package staffauth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Access verifies the sign-in proxy's JWT: Google Cloud Identity-Aware Proxy (IAP) or
// Cloudflare Access; any proxy that signs with RS256 or ES256 and publishes a JWKS works
// the same way.
type Access struct {
	// Header carries the token: X-Goog-Iap-Jwt-Assertion (IAP) or Cf-Access-Jwt-Assertion.
	Header   string
	Issuer   string // e.g. https://<team>.cloudflareaccess.com
	Audience string // the Access application's AUD tag
	JWKSURL  string // e.g. https://<team>.cloudflareaccess.com/cdn-cgi/access/certs
	// RequireAMR, if set, is an authentication method the token must report in its "amr"
	// claim, e.g. "hwk" for a hardware security key. Whether the claim arrives depends on
	// the identity provider; check a real token with "safety-svc admin check-token".
	RequireAMR string
	HTTP       *http.Client
	Now        func() time.Time

	mu      sync.Mutex
	keys    map[string]crypto.PublicKey
	fetched time.Time
}

var (
	ErrToken    = errors.New("staff token invalid")
	ErrNoKey    = errors.New("staff token signed with an unknown key")
	leeway      = 30 * time.Second
	minRefetch  = time.Minute
	maxKeyAge   = 6 * time.Hour
	maxTokenLen = 16 << 10
)

func (a *Access) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func b64(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

// keyFor returns the key for kid, fetching the JWKS when it's unknown or stale (at most
// once a minute, so a flood of bad tokens can't hammer the proxy).
func (a *Access) keyFor(kid string) (crypto.PublicKey, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	k, ok := a.keys[kid]
	stale := a.now().Sub(a.fetched) > maxKeyAge
	if ok && !stale {
		return k, nil
	}
	if a.now().Sub(a.fetched) < minRefetch && a.keys != nil {
		if ok {
			return k, nil
		}
		return nil, ErrNoKey
	}
	keys, err := a.fetch()
	if err != nil {
		if ok {
			return k, nil // keep serving with the cached key if the proxy is briefly down
		}
		return nil, err
	}
	a.keys, a.fetched = keys, a.now()
	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, ErrNoKey
}

func (a *Access) fetch() (map[string]crypto.PublicKey, error) {
	c := a.HTTP
	if c == nil {
		c = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := c.Get(a.JWKSURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("jwks: http " + resp.Status)
	}
	var set struct {
		Keys []struct {
			Kid, Kty, Alg, N, E, Crv, X, Y string
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&set); err != nil {
		return nil, err
	}
	out := map[string]crypto.PublicKey{}
	for _, k := range set.Keys {
		switch k.Kty {
		case "RSA":
			n, err1 := b64(k.N)
			e, err2 := b64(k.E)
			if err1 != nil || err2 != nil || len(n) < 256 { // ≥ 2048-bit
				continue
			}
			out[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
		case "EC":
			if k.Crv != "P-256" {
				continue
			}
			x, err1 := b64(k.X)
			y, err2 := b64(k.Y)
			if err1 != nil || err2 != nil || len(x) != 32 || len(y) != 32 {
				continue
			}
			pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), append(append([]byte{4}, x...), y...))
			if err == nil {
				out[k.Kid] = pub
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("jwks: no usable keys")
	}
	return out, nil
}

// Claims is what a verified token says about the staff member.
type Claims struct {
	Email string
	AMR   []string
}

// Verify checks a token and returns the staff email.
func (a *Access) Verify(token string) (string, error) {
	c, err := a.Inspect(token)
	return c.Email, err
}

// ErrAMR means the token is valid but doesn't show the required sign-in method.
var ErrAMR = errors.New("staff token doesn't show the required sign-in method")

// Inspect verifies a token and returns its claims.
func (a *Access) Inspect(token string) (Claims, error) {
	if len(token) > maxTokenLen {
		return Claims{}, ErrToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrToken
	}
	hb, err1 := b64(parts[0])
	cb, err2 := b64(parts[1])
	sig, err3 := b64(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return Claims{}, ErrToken
	}
	var h struct{ Alg, Kid string }
	if json.Unmarshal(hb, &h) != nil {
		return Claims{}, ErrToken
	}
	key, err := a.keyFor(h.Kid)
	if err != nil {
		return Claims{}, err
	}
	d := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	switch k := key.(type) {
	case *rsa.PublicKey:
		if h.Alg != "RS256" || rsa.VerifyPKCS1v15(k, crypto.SHA256, d[:], sig) != nil {
			return Claims{}, ErrToken
		}
	case *ecdsa.PublicKey:
		if h.Alg != "ES256" || len(sig) != 64 || !ecdsa.Verify(k, d[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
			return Claims{}, ErrToken
		}
	default:
		return Claims{}, ErrToken
	}
	var c struct {
		Iss   string          `json:"iss"`
		Aud   json.RawMessage `json:"aud"`
		Exp   int64           `json:"exp"`
		Nbf   int64           `json:"nbf"`
		Email string          `json:"email"`
		AMR   []string        `json:"amr"`
	}
	if json.Unmarshal(cb, &c) != nil {
		return Claims{}, ErrToken
	}
	now := a.now()
	if c.Iss != a.Issuer || !audOK(c.Aud, a.Audience) || c.Exp == 0 || now.After(time.Unix(c.Exp, 0).Add(leeway)) ||
		(c.Nbf != 0 && now.Add(leeway).Before(time.Unix(c.Nbf, 0))) || !strings.Contains(c.Email, "@") {
		return Claims{}, ErrToken
	}
	out := Claims{Email: strings.ToLower(c.Email), AMR: c.AMR}
	if a.RequireAMR != "" {
		ok := false
		for _, m := range c.AMR {
			if m == a.RequireAMR {
				ok = true
			}
		}
		if !ok {
			return out, ErrAMR
		}
	}
	return out, nil
}

func audOK(raw json.RawMessage, want string) bool {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one == want && want != ""
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		for _, a := range many {
			if a == want && want != "" {
				return true
			}
		}
	}
	return false
}

// CheckKeys fetches the published keys now and reports how many are usable (preflight).
func (a *Access) CheckKeys() (int, error) {
	keys, err := a.fetch()
	if err != nil {
		return 0, err
	}
	a.mu.Lock()
	a.keys, a.fetched = keys, a.now()
	a.mu.Unlock()
	return len(keys), nil
}
