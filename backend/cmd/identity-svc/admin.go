package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/dsr"
	"github.com/jnnseya-cpu/tryst/backend/internal/svcenv"
)

// runAdmin runs an operator command against the configured databases (same environment
// as the service). Commands:
//
//	verify             check a database set loads, the audit chain holds and photos decrypt
//	reapply-erasures   after a restore, erase again every account in the erasure ledger
//	                   (member platform databases; the safety service has its own command)
//
// Output is JSON with counts only; no member data is printed.
func runAdmin(a *app, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: identity-svc admin preflight|verify|reapply-erasures")
		return 2
	}
	ctx := context.Background()
	var out any
	var err error
	switch args[0] {
	case "verify":
		out, err = verify(ctx, a)
	case "reapply-erasures":
		out, err = reapplyErasures(ctx, a)
	case "preflight":
		var ok bool
		out, ok = preflightPlatform(ctx, a)
		if !ok {
			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(b))
			return 1
		}
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "admin:", err)
		return 1
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
	return 0
}

// verify loads every service (which already checks the audit hash chain), counts records
// by kind, and opens every member's photos with the configured key.
func verify(ctx context.Context, a *app) (any, error) {
	counts := map[string]map[string]int{}
	kinds := map[string][]string{
		"identity": {"identity", "session", "proof"}, "profile": {"profile", "thread", "meet", "inbox"},
		"keys": {"datakey"}, "commerce": {"account", "event", "voucher"}, "ledger": {"erasure"},
	}
	for db, ks := range kinds {
		counts[db] = map[string]int{}
		for _, k := range ks {
			docs, err := a.stores[db].All(ctx, k)
			if err != nil {
				return nil, fmt.Errorf("%s/%s: %w", db, k, err)
			}
			counts[db][k] = len(docs)
		}
	}
	profiles, err := a.stores["profile"].All(ctx, "profile")
	if err != nil {
		return nil, err
	}
	photos, unreadable := 0, 0
	for id := range profiles {
		ex, err := a.prof.Export(ctx, id)
		if err != nil {
			return nil, err
		}
		m, _ := ex.(map[string]any)
		list, _ := m["photos"].([]map[string]any)
		for _, p := range list {
			photos++
			if p["data_base64"] == nil {
				unreadable++
			}
		}
	}
	ok := unreadable == 0
	return map[string]any{"ok": ok, "records": counts, "photos": photos, "photos_unreadable": unreadable, "audit_chain": "verified on load"}, nil
}

// reapplyErasures erases, again, every account whose subject code is in the erasure ledger.
// Run it after every restore, before the service takes traffic (docs/runbooks/restore.md).
func reapplyErasures(ctx context.Context, a *app) (any, error) {
	entries, err := a.stores["ledger"].All(ctx, "erasure")
	if err != nil {
		return nil, err
	}
	erased := map[string]bool{}
	for _, b := range entries {
		var e dsr.LedgerEntry
		if json.Unmarshal(b, &e) == nil {
			erased[e.Subject] = true
		}
	}
	candidates := map[string]bool{}
	for db, k := range map[string]string{"identity": "identity", "profile": "profile", "keys": "datakey", "commerce": "account"} {
		docs, err := a.stores[db].All(ctx, k)
		if err != nil {
			return nil, err
		}
		for id := range docs {
			candidates[id] = true
		}
	}
	inbox, err := a.stores["profile"].All(ctx, "inbox")
	if err != nil {
		return nil, err
	}
	for id := range inbox {
		candidates[id] = true
	}
	var ids []string
	for id := range candidates {
		if erased[dsr.SubjectCode(id)] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	a.commerce.SkipProcessor = true
	// The safety service has its own database and runs its own reapply-erasures
	// (cmd/safety-svc admin), so a restore here never depends on it being up.
	parts := []dsr.Part{a.prof, a.commerce, a.notifier, a.srv}
	for _, id := range ids {
		for _, p := range parts {
			if _, err := p.Erase(ctx, id); err != nil {
				return nil, fmt.Errorf("%s: %w", p.Name(), err)
			}
		}
	}
	return map[string]any{"ledger_entries": len(erased), "accounts_checked": len(candidates), "erased_again": len(ids)}, nil
}

// preflightPlatform checks the member platform's side of the safety-service link: two
// distinct internal secrets and a signed round trip to the safety service.
func preflightPlatform(ctx context.Context, a *app) (map[string]any, bool) {
	s1, s2 := svcenv.InternalSecret(a.mode, "INTERNAL_SECRET_SAFETY"), svcenv.InternalSecret(a.mode, "INTERNAL_SECRET_PLATFORM")
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := a.toSafety.Do(pctx, "GET", "/internal/v1/ping", nil, nil)
	pending := a.signals.Pending(ctx)
	out := map[string]any{
		"internal_secrets_distinct": !bytes.Equal(s1, s2),
		"safety_service_reachable":  err == nil,
		"signals_queued":            pending,
	}
	if err != nil {
		out["safety_service_error"] = err.Error()
	}
	return out, !bytes.Equal(s1, s2) && err == nil
}
