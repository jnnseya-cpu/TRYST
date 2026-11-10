package growth

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// BriefFields is the closed Growth Brief v1 object (shared/contracts/growth/
// growth-brief.v1.schema.yaml, Appendix A of the build spec). contract_test.go checks this
// list and every fixture against the schema file.
var BriefFields = []string{"schema_version", "brief_id", "brief_version", "tenant_id", "platform_id", "market", "locale",
	"topic_id", "opportunity_score", "confidence", "valid_from", "expires_at", "evidence_ids", "audience", "intent",
	"approved_claim_ids", "cta_id", "allowed_channels", "prohibited_claims", "objective", "experiment_id", "policy_version"}

var utcRe = regexp.MustCompile(`Z$`)

func num(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	}
	return 0, false
}

func list(v any) ([]any, bool) {
	switch x := v.(type) {
	case []any:
		return x, true
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out, true
	}
	return nil, false
}

// BriefSchemaErrors validates a brief against the v1 schema: closed object, required
// fields, types, patterns, bounds and unique lists.
func BriefSchemaErrors(b map[string]any) []string {
	var errs []string
	for k := range b {
		if !slices.Contains(BriefFields, k) {
			errs = append(errs, k+": not allowed")
		}
	}
	for _, k := range BriefFields {
		if _, ok := b[k]; !ok {
			errs = append(errs, k+": required")
		}
	}
	str := func(k string, min, max int, re *regexp.Regexp) {
		v, ok := b[k].(string)
		if _, present := b[k]; !present {
			return
		}
		if !ok || len([]rune(v)) < min || len([]rune(v)) > max || (re != nil && !re.MatchString(v)) {
			errs = append(errs, k+": invalid")
		}
	}
	intMin := func(k string, min float64) {
		if _, present := b[k]; !present {
			return
		}
		n, ok := num(b[k])
		if !ok || n != float64(int64(n)) || n < min {
			errs = append(errs, k+": must be an integer ≥ "+fmt.Sprint(min))
		}
	}
	score := func(k string) {
		if _, present := b[k]; !present {
			return
		}
		if n, ok := num(b[k]); !ok || n < 0 || n > 100 {
			errs = append(errs, k+": must be 0-100")
		}
	}
	ids := func(k string, minItems, maxItems int, allowed []string, item func(string) bool) {
		if _, present := b[k]; !present {
			return
		}
		l, ok := list(b[k])
		if !ok || len(l) < minItems || len(l) > maxItems {
			errs = append(errs, k+": wrong number of items")
			return
		}
		seen := map[string]bool{}
		for _, x := range l {
			s, ok := x.(string)
			if !ok || !item(s) || (allowed != nil && !slices.Contains(allowed, s)) || seen[s] {
				errs = append(errs, k+": invalid or duplicate item")
				return
			}
			seen[s] = true
		}
	}
	if n, ok := num(b["schema_version"]); !ok || n != 1 {
		errs = append(errs, "schema_version: must be 1")
	}
	for _, k := range []string{"brief_id", "tenant_id", "platform_id", "topic_id", "cta_id", "experiment_id"} {
		str(k, 1, 128, idRe)
	}
	intMin("brief_version", 1)
	intMin("policy_version", 1)
	str("market", 2, 2, regexp.MustCompile(`^[A-Z]{2}$`))
	str("locale", 2, 35, nil)
	score("opportunity_score")
	score("confidence")
	for _, k := range []string{"valid_from", "expires_at"} {
		str(k, 1, 40, utcRe)
		if s, ok := b[k].(string); ok {
			if _, err := time.Parse(time.RFC3339, s); err != nil {
				errs = append(errs, k+": must be an RFC 3339 UTC time")
			}
		}
	}
	for _, k := range []string{"audience", "intent", "objective"} {
		str(k, 1, 500, nil)
	}
	isID := func(s string) bool { return idRe.MatchString(s) }
	ids("evidence_ids", 1, 64, nil, isID)
	ids("approved_claim_ids", 0, 64, nil, isID)
	ids("allowed_channels", 1, 6, Channels, func(string) bool { return true })
	ids("prohibited_claims", 0, 32, nil, func(s string) bool { return len([]rune(s)) >= 1 && len([]rune(s)) <= 500 })
	sort.Strings(errs)
	return slices.Compact(errs)
}

// errsText joins errors for logs.
func errsText(e []string) string { return strings.Join(e, "; ") }
