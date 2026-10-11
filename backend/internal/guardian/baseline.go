// Package guardian is the deterministic baseline for the message-safety signals
// M6f (romance and financial fraud), M6g (child safety) and M6h (exposure extortion)
// (docs/spec/03_Backend.md §7.7, FR-102, FR-110, D-47).
//
// It runs on the member's device (messages are end-to-end encrypted; the server never sees
// them). This Go version is the reference implementation; the iOS, Android and web ports use
// the same patterns and must pass the same labelled vectors (contracts/vocab/guardian_vectors.json).
// Trained models replace a rule only when they beat it on the release gate (§7.7 D).
package guardian

import (
	"regexp"
	"strings"
)

// Signal is a fired category and the action the client takes.
type Signal struct {
	Kind   string // fraud | child_safety | extortion
	Action string // warn_recipient | freeze_urgent | freeze_support
}

func re(s string) *regexp.Regexp { return regexp.MustCompile(`(?i)` + s) }

var (
	money = `(?:money|cash|£\s?\d+|\$\s?\d+|\d+\s?(?:quid|pounds|gbp)|funds|rent)`
	fraud = []*regexp.Regexp{
		re(`\b(?:send|transfer|wire|lend|loan|give)\s+(?:me\s+)?(?:some\s+)?` + money),
		re(`\b(?:transfer|send|wire)\s+(?:me\s+)?(?:the\s+)?money\b`),
		re(`\b(?:gift|steam|itunes|apple|amazon|google\s*play)\s*cards?\b`),
		re(`\b(?:crypto|bitcoin|btc|usdt|ethereum)\b.{0,60}\b(?:invest|platform|wallet|returns?|profit|trading)\b`),
		re(`\b(?:invest(?:ment)?|trading)\s+(?:platform|opportunity|app)\b`),
		re(`\bguaranteed\s+(?:returns?|profit)\b`),
		re(`\bwallet\s+address\b`),
		re(`\b(?:western\s+union|moneygram)\b`),
		re(`\bpay\s+(?:for\s+)?my\s+(?:ticket|flight|taxi|visa|hospital|bills?|fine)\b`),
	}

	child    = `(?:kids?|child(?:ren)?|son|daughter|little\s+(?:girl|boy)|step-?(?:son|daughter))`
	childPat = []*regexp.Regexp{
		re(`\b(?:pics?|photos?|pictures?|videos?)\s+of\s+(?:your\s+)?` + child + `\b`),
		re(`\b(?:your\s+)?` + child + `\b.{0,25}\b(?:home\s+)?alone\b`),
		re(`\bbring\s+(?:your\s+)?` + child + `\b`),
		re(`\bbabysit\b.{0,40}\b(?:just\s+me|alone|them)\b`),
		re(`\bpretend\s+(?:you'?re|to\s+be)\s+(?:1[0-7]|[1-9])\b`),
		re(`\bhow\s+old\s+is\s+your\s+` + child + `\b.{0,40}\b(?:pretty|cute|hot|sexy|developed)\b`),
		re(`\b(?:like|want|into)\s+(?:them\s+)?(?:young|little)\b.{0,20}\b(?:1[0-7]|[1-9])\b`),
		re(`\b(?:underage|preteen|pre-teen|jailbait)\b`),
	}

	who        = `(?:wife|husband|partner|boss|employer|work|family|kids|friends|everyone|church|mum|dad)`
	extortPats = []*regexp.Regexp{
		re(`\b(?:pay|send|transfer)\b.{0,40}\b(?:or|unless|otherwise)\b.{0,40}\b(?:tell|show|send|go\s+to|post)\b.{0,30}\byour\s+` + who),
		re(`\b(?:tell|show|send)\b.{0,15}\byour\s+` + who + `\b.{0,60}\b(?:unless|if\s+you\s+don'?t|or\s+else)\b`),
		re(`\b(?:unless|if\s+you\s+don'?t|or\s+(?:i'?ll|i\s+will))\b.{0,60}\b(?:tell|show|send)\b.{0,15}\byour\s+` + who),
		re(`\b(?:pay|send|money|£|cash)\b.{0,40}\bunless\b.{0,30}\byour\s+` + who + `\b.{0,30}\b(?:see|know|find\s+out|hear)\b`),
		re(`\bexpose\s+you\b`),
		re(`\beveryone\s+will\s+know\b`),
		re(`\b(?:pics|photos|screenshots|videos)\b.{0,30}\b(?:go|going|sent)\s+to\s+your\s+` + who),
		re(`\bi\s+know\s+where\s+you\s+(?:work|live)\b`),
	}
)

func normalise(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("’", "'", "‘", "'", "\u200b", "", "\u200d", "").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func any(text string, ps []*regexp.Regexp) bool {
	for _, p := range ps {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

// Check returns the signals one message fires. Order is fixed: child safety first.
func Check(message string) []Signal {
	s := normalise(message)
	var out []Signal
	if any(s, childPat) {
		out = append(out, Signal{"child_safety", "freeze_urgent"})
	}
	if any(s, extortPats) {
		out = append(out, Signal{"extortion", "freeze_support"})
	}
	if any(s, fraud) {
		out = append(out, Signal{"fraud", "warn_recipient"})
	}
	return out
}
