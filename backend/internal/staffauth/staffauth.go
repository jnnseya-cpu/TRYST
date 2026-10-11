// Package staffauth authenticates TRYST staff for the moderator console and other ops
// endpoints (03 §7.3; D-59). Staff never sign in to TRYST itself: the console host sits
// behind the company sign-in proxy, and TRYST checks what the proxy asserts.
//
//   - Access (production, preferred): the proxy's signed token (Cloudflare Access sends it
//     as Cf-Access-Jwt-Assertion) is verified against the proxy's published keys, issuer,
//     audience and expiry. The staff member is the token's email.
//   - Proxy headers: X-Staff-Identity from an oauth2-proxy-style gateway, believed only from
//     its addresses (TrustedProxies).
//   - Dev tokens: "Authorization: Staff <token>", hashed; refused outside dev.
//
// Roles come from the staff roster (who does T&S work), never from the request, so
// belonging to the company isn't enough to open the console.
package staffauth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
)

// Staff is an authenticated staff member.
type Staff struct {
	ID    string
	Roles []string
}

// Has reports whether the staff member holds a role.
func (s Staff) Has(role string) bool {
	for _, r := range s.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Auth authenticates console requests.
type Auth struct {
	Env            string
	Access         *Access             // verified proxy token (production)
	TrustedProxies []string            // addresses/CIDRs of a header-asserting proxy
	Roster         map[string][]string // staff id (email) -> roles
	DevTokens      map[string]Staff    // sha256(token) hex -> staff; dev only
}

var ErrDevTokensInProd = errors.New("staff dev tokens are forbidden outside the dev environment")

// Check validates the configuration.
func (a Auth) Check() error {
	if a.Env != "dev" && len(a.DevTokens) > 0 {
		return ErrDevTokensInProd
	}
	if a.Env != "dev" && (a.Access != nil || len(a.TrustedProxies) > 0) && len(a.Roster) == 0 {
		return errors.New("staff sign-in needs a roster (STAFF_ROSTER) outside dev")
	}
	return nil
}

func (a Auth) withRoles(id string, headerRoles string) (Staff, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return Staff{}, false
	}
	if a.Roster != nil {
		roles, ok := a.Roster[id]
		if !ok {
			return Staff{}, false
		}
		return Staff{ID: id, Roles: roles}, true
	}
	if a.Env != "dev" {
		return Staff{}, false // roles from a request are only believed in dev
	}
	return Staff{ID: id, Roles: strings.Split(headerRoles, ",")}, true
}

// Authenticate returns the staff member for a request, if any.
func (a Auth) Authenticate(r *http.Request) (Staff, bool) {
	if a.Access != nil {
		if tok := r.Header.Get("Cf-Access-Jwt-Assertion"); tok != "" {
			if email, err := a.Access.Verify(tok); err == nil {
				return a.withRoles(email, "")
			}
			return Staff{}, false
		}
	}
	if len(a.TrustedProxies) > 0 {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if identity.TrustedAddr(a.TrustedProxies, ip) {
			if id := r.Header.Get("X-Staff-Identity"); id != "" {
				return a.withRoles(id, r.Header.Get("X-Staff-Roles"))
			}
		}
	}
	if a.Env == "dev" {
		if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Staff "); ok {
			h := sha256.Sum256([]byte(tok))
			if s, ok := a.DevTokens[hex.EncodeToString(h[:])]; ok {
				return s, true
			}
		}
	}
	return Staff{}, false
}

// ParseRoster reads "email:role,role;email:role".
func ParseRoster(s string) (map[string][]string, error) {
	out := map[string][]string{}
	for _, entry := range strings.Split(s, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		email, roles, ok := strings.Cut(entry, ":")
		if !ok || !strings.Contains(email, "@") || roles == "" {
			return nil, errors.New("STAFF_ROSTER entries are email:role[,role]")
		}
		for _, r := range strings.Split(roles, ",") {
			switch r {
			case "moderator", "lead":
			default:
				return nil, errors.New("unknown staff role " + r)
			}
		}
		out[strings.ToLower(strings.TrimSpace(email))] = strings.Split(roles, ",")
	}
	return out, nil
}

// HashToken is how dev tokens are stored.
func HashToken(tok string) string {
	h := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(h[:])
}

// FromEnv builds the staff sign-in from the environment:
//
//	STAFF_ACCESS_ISSUER, STAFF_ACCESS_AUDIENCE, STAFF_ACCESS_JWKS_URL  verified proxy token
//	STAFF_TRUSTED_PROXIES  addresses of a header-asserting proxy (alternative)
//	STAFF_ROSTER           email:role[,role];...  (required outside dev)
//	STAFF_DEV_TOKENS       id:role:token,...       (dev only)
func FromEnv(mode string, getenv func(string) string) (Auth, error) {
	a := Auth{Env: mode, DevTokens: map[string]Staff{}}
	if iss := getenv("STAFF_ACCESS_ISSUER"); iss != "" {
		a.Access = &Access{Issuer: iss, Audience: getenv("STAFF_ACCESS_AUDIENCE"), JWKSURL: getenv("STAFF_ACCESS_JWKS_URL")}
		if a.Access.Audience == "" || !strings.HasPrefix(a.Access.JWKSURL, "https://") {
			return a, errors.New("STAFF_ACCESS_AUDIENCE and an https STAFF_ACCESS_JWKS_URL are required with STAFF_ACCESS_ISSUER")
		}
	}
	if v := getenv("STAFF_TRUSTED_PROXIES"); v != "" {
		a.TrustedProxies = strings.Split(v, ",")
	}
	if v := getenv("STAFF_ROSTER"); v != "" {
		r, err := ParseRoster(v)
		if err != nil {
			return a, err
		}
		a.Roster = r
	}
	if v := getenv("STAFF_DEV_TOKENS"); v != "" {
		for _, item := range strings.Split(v, ",") {
			parts := strings.SplitN(item, ":", 3)
			if len(parts) != 3 || len(parts[2]) < 16 {
				return a, errors.New("STAFF_DEV_TOKENS entries are id:role:token with a token of 16+ characters")
			}
			a.DevTokens[HashToken(parts[2])] = Staff{ID: parts[0], Roles: []string{parts[1]}}
		}
	}
	return a, a.Check()
}
