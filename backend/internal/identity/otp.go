package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// OTPs prove ownership of a contact handle at sign-up only. They are never a login factor
// (FR-056).
const (
	otpTTL         = 10 * time.Minute
	otpMaxAttempts = 5
	otpMaxSends    = 5 // per contact per hour
)

// Sender delivers OTPs (SMS/email gateway in production).
type Sender interface {
	Send(contact, code string) error
}

// NormaliseContact lower-cases emails and strips spaces from phone numbers.
func NormaliseContact(c string) string {
	c = strings.TrimSpace(c)
	if strings.Contains(c, "@") {
		return strings.ToLower(c)
	}
	return strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(c)
}

// ValidContact performs a light syntactic check.
func ValidContact(c string) bool {
	if strings.Contains(c, "@") {
		at := strings.LastIndex(c, "@")
		return at > 0 && strings.Contains(c[at:], ".") && len(c) <= 254
	}
	if !strings.HasPrefix(c, "+") || len(c) < 8 || len(c) > 16 {
		return false
	}
	for _, r := range c[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func contactHash(pepper []byte, contact string) string {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(NormaliseContact(contact)))
	return hex.EncodeToString(m.Sum(nil))
}

func newOTP() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%06d", n.Int64())
}
