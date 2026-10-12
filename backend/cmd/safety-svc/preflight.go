package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jnnseya-cpu/tryst/backend/internal/staffauth"
	"os"
	"strings"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/svcenv"
)

type check struct {
	Name   string `json:"check"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// preflight checks the production setup before the service takes traffic (docs/runbooks/
// deploy.md): the staff roster can run the two-person rule and independent appeals, the
// sign-in proxy's keys are reachable, the internal secrets are distinct, and the database
// and the member platform answer. It prints no secrets and no roster emails.
func preflight(ctx context.Context, a *app) ([]check, bool) {
	var out []check
	add := func(name string, ok bool, format string, args ...any) {
		out = append(out, check{name, ok, fmt.Sprintf(format, args...)})
	}

	// Roster: a permanent removal needs a proposer and a different confirmer, and an appeal
	// needs a reviewer who was neither, so at least three people; at least one lead.
	staff, leads := 0, 0
	for _, roles := range a.staff.Roster {
		mod := false
		for _, r := range roles {
			if r == "lead" {
				leads++
				mod = true
			}
			if r == "moderator" {
				mod = true
			}
		}
		if mod {
			staff++
		}
	}
	switch {
	case a.mode == "dev" && len(a.staff.Roster) == 0:
		add("staff roster", true, "dev: no roster (dev tokens: %d)", len(a.staff.DevTokens))
	default:
		add("staff roster", staff >= 3 && leads >= 1, "%d staff, %d leads (need ≥ 3 staff, ≥ 1 lead: proposer, second moderator and an uninvolved appeal reviewer)", staff, leads)
	}

	// Sign-in proxy.
	switch {
	case a.staff.Access != nil:
		n, err := a.staff.Access.CheckKeys()
		add("sign-in proxy keys", err == nil && n > 0, "%d usable keys from %s%s", n, a.staff.Access.JWKSURL, errText(err))
		if a.staff.Access.RequireAMR != "" {
			add("hardware key", true, "tokens must show sign-in method %q", a.staff.Access.RequireAMR)
		} else if a.staff.Access.Header == staffauth.HeaderIAP {
			// IAP tokens don't say how someone signed in: the operator confirms Google's policy.
			add("hardware key", a.mode == "dev" || os.Getenv("STAFF_ACCESS_KEYS_CONFIRMED") == "google-2sv-security-keys-only",
				"not visible to TRYST with IAP: set the safety team's Google 2-Step Verification to security keys only, then STAFF_ACCESS_KEYS_CONFIRMED=google-2sv-security-keys-only")
		} else {
			add("hardware key", a.mode == "dev", "not checked by TRYST: the Access policy must require a hardware key (or set STAFF_ACCESS_REQUIRE_AMR=hwk after confirming with check-token)")
		}
	case len(a.staff.TrustedProxies) > 0:
		add("sign-in proxy keys", a.mode == "dev", "header proxy mode (%d addresses); prefer STAFF_ACCESS_* so the proxy's assertion is verified, not trusted by address", len(a.staff.TrustedProxies))
	default:
		add("sign-in proxy keys", a.mode == "dev", "dev tokens only")
	}

	// Internal secrets: one per direction, never the same value.
	s1, s2 := svcenv.InternalSecret(a.mode, "INTERNAL_SECRET_SAFETY"), svcenv.InternalSecret(a.mode, "INTERNAL_SECRET_PLATFORM")
	add("internal secrets", !bytes.Equal(s1, s2), "two distinct secrets, %d and %d bytes", len(s1), len(s2))
	rot := []string{}
	for _, v := range []string{"INTERNAL_SECRET_SAFETY", "INTERNAL_SECRET_PLATFORM"} {
		if svcenv.PreviousSecret(v) != nil {
			rot = append(rot, v+"_PREVIOUS")
		}
	}
	add("rotation", true, "previous secrets accepted: %v (remove once both services use the new ones)", rot)

	// Database and the member platform.
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if p, ok := a.st.(interface{ Ping(context.Context) error }); ok {
		err := p.Ping(pctx)
		add("safety database", err == nil, "reachable%s", errText(err))
	} else {
		add("safety database", a.mode == "dev", "in-memory (dev only)")
	}
	err := a.toPlatform.Do(pctx, "GET", "/internal/v1/ping", nil, nil)
	add("member platform", err == nil, "signed ping to %s%s", a.toPlatform.BaseURL, errText(err))

	all := true
	for _, c := range out {
		all = all && c.OK
	}
	return out, all
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return ": " + err.Error()
}

// checkToken verifies a staff token pasted on stdin (copy it from the CF_Authorization
// cookie after signing in to the console host) and shows what TRYST would conclude.
func checkToken(a *app) int {
	if a.staff.Access == nil {
		fmt.Fprintln(os.Stderr, "admin: STAFF_ACCESS_* is not configured")
		return 2
	}
	tok, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	c, err := a.staff.Access.Inspect(strings.TrimSpace(tok))
	roles, onRoster := a.staff.Roster[c.Email]
	res := map[string]any{"valid": err == nil, "email": c.Email, "sign_in_methods": c.AMR, "on_roster": onRoster, "roles": roles}
	if err != nil {
		res["error"] = err.Error()
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(b))
	if err != nil || !onRoster {
		return 1
	}
	return 0
}
