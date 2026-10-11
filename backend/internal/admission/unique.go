package admission

// One person, one account (FR-097, D-41), and control-network detection.
//
// The ban store keeps banned people out; this index keeps everyone else to a single active
// account and spots accounts controlled from the same device or card, a pattern of scam
// farms, pimping and trafficking (one controller, many profiles). It works on the same
// one-way anchors (HMACs): no ID number, device ID or card number is ever stored.

import "encoding/hex"

// Verdict for a new or re-verified account.
type Verdict struct {
	Allowed bool
	Review  []string // reasons a person in Trust & Safety must look at the account
}

// Index holds the anchors of active (not erased) accounts.
type Index struct {
	doc     map[string]string          // document anchor → account
	device  map[string]map[string]bool // device anchor → accounts
	payment map[string]map[string]bool // payment anchor → accounts
}

func NewIndex() *Index {
	return &Index{doc: map[string]string{}, device: map[string]map[string]bool{}, payment: map[string]map[string]bool{}}
}

// MaxAccountsPerCard is the most accounts a single card may pay for: a member and their
// verified couple partner. A third account on the same card is refused.
const MaxAccountsPerCard = 2

func k(b []byte) string { return hex.EncodeToString(b) }

// Check decides without changing the index. partner is the account's verified couple
// partner, if any (couples may share a card, never a phone or an ID).
func (x *Index) Check(account string, a Anchors, partner string) Verdict {
	v := Verdict{Allowed: true}
	if len(a.Doc) == 0 {
		return Verdict{Allowed: false, Review: []string{"no_document_anchor"}}
	}
	if other, ok := x.doc[k(a.Doc)]; ok && other != account {
		// Same ID document already holds an active account: refuse silently (a person
		// who has lost access recovers their account; they do not open a second one).
		return Verdict{Allowed: false, Review: []string{"second_account_same_document"}}
	}
	for _, d := range a.Devices {
		for other := range x.device[k(d)] {
			if other != account {
				v.Review = append(v.Review, "device_shared_with_other_account")
			}
		}
	}
	if len(a.Payment) > 0 {
		others := 0
		for other := range x.payment[k(a.Payment)] {
			if other != account {
				others++
				if other != partner {
					v.Review = append(v.Review, "card_shared_with_non_partner")
				}
			}
		}
		if others+1 > MaxAccountsPerCard {
			v.Allowed = false
			v.Review = append(v.Review, "card_pays_for_too_many_accounts")
		}
	}
	return v
}

// Register adds an allowed account to the index and returns the verdict.
func (x *Index) Register(account string, a Anchors, partner string) Verdict {
	v := x.Check(account, a, partner)
	if !v.Allowed {
		return v
	}
	x.doc[k(a.Doc)] = account
	for _, d := range a.Devices {
		if x.device[k(d)] == nil {
			x.device[k(d)] = map[string]bool{}
		}
		x.device[k(d)][account] = true
	}
	if len(a.Payment) > 0 {
		if x.payment[k(a.Payment)] == nil {
			x.payment[k(a.Payment)] = map[string]bool{}
		}
		x.payment[k(a.Payment)][account] = true
	}
	return v
}

// Remove drops an account on erasure (its anchor candidates are destroyed with it unless
// a permanent ban copied them to the ban store).
func (x *Index) Remove(account string, a Anchors) {
	if x.doc[k(a.Doc)] == account {
		delete(x.doc, k(a.Doc))
	}
	for _, d := range a.Devices {
		delete(x.device[k(d)], account)
	}
	if len(a.Payment) > 0 {
		delete(x.payment[k(a.Payment)], account)
	}
}
