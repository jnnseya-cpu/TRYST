// Package admission implements L1/L2 admission control (docs/spec/03_Backend.md §8;
// FR-036, E-02). One-way anchors are computed at V2 for every member (anchor candidates,
// erased with the account) and copied to the ban store only on a human-decided permanent
// ban - the declared exception to erasure (ToS cl. 15.3).
package admission

import (
	"crypto/hmac"
	"crypto/sha256"
	"strings"
)

// Pepper stands in for the HSM-held, non-exportable ban pepper.
type Pepper []byte

// Anchors are irreversible identifiers. No document, device id or card token is stored.
type Anchors struct {
	Doc     []byte
	Devices [][]byte
	Payment []byte
}

func mac(p Pepper, domain, value string) []byte {
	m := hmac.New(sha256.New, p)
	m.Write([]byte(domain))
	m.Write([]byte{0})
	m.Write([]byte(strings.TrimSpace(strings.ToUpper(value))))
	return m.Sum(nil)
}

// Derive computes anchors from provider-returned values at V2. Inputs are discarded after.
func Derive(p Pepper, docNumber string, attestationIDs []string, paymentToken string) Anchors {
	a := Anchors{Doc: mac(p, "doc", docNumber)}
	for _, id := range attestationIDs {
		a.Devices = append(a.Devices, mac(p, "device", id))
	}
	if paymentToken != "" {
		a.Payment = mac(p, "payment", paymentToken)
	}
	return a
}

// BanStore holds anchors of permanently removed members (BAN-DB).
type BanStore struct{ entries []Anchors }

// Ban records anchors. Callers must supply the deciding staff member: permanent bans are
// always a named human decision (FR-035).
func (b *BanStore) Ban(a Anchors, reviewedBy string) bool {
	if strings.TrimSpace(reviewedBy) == "" {
		return false
	}
	b.entries = append(b.entries, a)
	return true
}

// Lift destroys anchors after a successful appeal (ToS 18.2).
func (b *BanStore) Lift(a Anchors) {
	kept := b.entries[:0]
	for _, e := range b.entries {
		if !hmac.Equal(e.Doc, a.Doc) {
			kept = append(kept, e)
		}
	}
	b.entries = kept
}

// Admit is checked before account creation and again at V2. It returns only a boolean so
// the caller can refuse silently; it never reveals which anchor matched (it teaches evasion).
func (b *BanStore) Admit(a Anchors) bool {
	for _, e := range b.entries {
		if len(a.Doc) > 0 && hmac.Equal(e.Doc, a.Doc) {
			return false
		}
		if len(a.Payment) > 0 && len(e.Payment) > 0 && hmac.Equal(e.Payment, a.Payment) {
			return false
		}
		for _, d := range a.Devices {
			for _, ed := range e.Devices {
				if hmac.Equal(d, ed) {
					return false
				}
			}
		}
	}
	return true
}
