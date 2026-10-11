package profile

// Screening of profile text: display name, bio and couple description (FR-098).
//
// Profile text is public-tier (seen by other members, not end-to-end encrypted), so the
// server screens it before it is shown. The same rules and test vectors are used by the
// on-device Guardian for messages, which the server never sees. Rules are deterministic so
// a member gets a clear reason; models (03 §7) handle what rules cannot.

import (
	"regexp"
	"strings"
)

// Finding is one problem in a piece of text.
type Finding struct {
	Kind   string
	Action string // "block": the text is refused; "urgent": refused and sent to T&S within 15 min
}

var (
	// Phone numbers, also written with spaces, dots or dashes between digits.
	rePhone  = regexp.MustCompile(`(?:\+?\d[\s.\-()]*){9,}`)
	reEmail  = regexp.MustCompile(`[a-z0-9._%+\-]+\s*(?:@|\(at\)|\[at\])\s*[a-z0-9.\-]+\s*(?:\.|\(dot\)|\[dot\])\s*[a-z]{2,}`)
	reURL    = regexp.MustCompile(`(?:https?://|www\.)\S+|\b[a-z0-9\-]+\.(?:com|net|org|io|me|co|uk|xyz|link|ly|gg|app)\b`)
	reHandle = regexp.MustCompile(`(?:^|\s)@[a-z0-9_.]{3,}|\b(?:snap(?:chat)?|insta(?:gram)?|ig|telegram|tg|whats\s*app|wa|kik|signal|onlyfans|of|fansly|discord|wickr|session)\s*[:\-]?\s*@?[a-z0-9_.]{3,}`)
	// Payment and crypto details.
	rePay = regexp.MustCompile(`\b(?:paypal(?:\.me)?|cash\s*app|\$[a-z][a-z0-9_]{2,}|revolut|monzo\s*me|venmo|iban|sort\s*code|bitcoin|btc|eth(?:ereum)?|usdt|crypto\s*wallet|gift\s*cards?)\b|\b(?:bc1|[13])[a-zA-HJ-NP-Z0-9]{25,39}\b|\b0x[a-f0-9]{40}\b`)
	// Commercial sexual services (01 §4.1: zero tolerance).
	reSale = regexp.MustCompile(`\b(?:in\s*call|out\s*call|incall|outcall|ppm|gfe|pse|roses|donations|my\s*rates|rates\s*(?:are|from|start)|per\s*hour|hourly\s*rate|£\s*\d+|\d+\s*(?:gbp|quid|pounds)\b|escort(?:ing)?|sugar\s*(?:daddy|baby)|monthly\s*allowance|pay\s*(?:to|for)\s*play|findom|wishlist)\b`)
	// Drug supply (harm-reduction talk is not blocked; selling is).
	reDrugs = regexp.MustCompile(`\b(?:selling|supplier|plug|delivery|dm\s*for\s*menu|menu\s*available)\b.{0,40}\b(?:tina|crystal|meth|g\b|ghb|gbl|mdma|molly|coke|charlie|ket|ketamine|weed|pills)\b|\b(?:tina|ghb|gbl|mdma|coke|ket)\b.{0,40}\b(?:for\s*sale|available|delivered|£)`)
	// Anything suggesting a minor or sexualising children. Never allowed in any context.
	reMinor = regexp.MustCompile(`\b(?:1[0-7]|[1-9])\s*(?:yo|y/o|yrs?\s*old|years?\s*old)\b|\bi(?:'m|\s*am)\s*1[0-7](?:\s*(?:yo|y/o|years?|yrs?|and|but|,|\.|!)|$)|\b(?:school\s*girl|school\s*boy|under\s*age|underage|barely\s*legal|jailbait|lolita|young\s*teen|preteen|pre-teen|minor)\b|\bteen\b`)
)

var leet = strings.NewReplacer("0", "o", "1", "i", "3", "e", "4", "a", "5", "s", "7", "t", "@", "a", "$", "s")

// normalise lower-cases, folds common look-alike characters and collapses spacing tricks.
func normalise(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("０", "0", "１", "1", "２", "2", "３", "3", "４", "4", "５", "5", "６", "6", "７", "7", "８", "8", "９", "9",
		"＠", "@", "．", ".", "\u200b", "", "\u200c", "", "\u200d", "", "\u2060", "").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// ScreenText returns every finding. Empty means the text may be shown.
func ScreenText(raw string) []Finding {
	s := normalise(raw)
	letters := leet.Replace(s) // for word lists; digits stay in s for number checks
	var out []Finding
	add := func(kind, action string) {
		for _, f := range out {
			if f.Kind == kind {
				return
			}
		}
		out = append(out, Finding{kind, action})
	}
	if reMinor.MatchString(s) || reMinor.MatchString(letters) {
		add("minor_reference", "urgent")
	}
	if rePhone.MatchString(s) || reEmail.MatchString(s) || reURL.MatchString(s) || reHandle.MatchString(s) {
		add("contact_details", "block")
	}
	if rePay.MatchString(s) || rePay.MatchString(letters) {
		add("payment_details", "block")
	}
	if reSale.MatchString(s) || reSale.MatchString(letters) {
		add("commercial_sex", "urgent")
	}
	if reDrugs.MatchString(s) || reDrugs.MatchString(letters) {
		add("drug_supply", "urgent")
	}
	return out
}
