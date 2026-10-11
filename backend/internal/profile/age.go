// Package profile holds the rules for what other members see on a profile and what may be
// uploaded to it (docs/spec/02_Shared_Contracts.md §1, §4.3; 03 §7.5; FR-092–FR-094,
// D-37–D-39).
package profile

import (
	"errors"
	"fmt"
	"time"
)

// BirthYM is the verified birth year and month, read from the identity document by the
// age-assurance provider at V2. The full date of birth is never stored (minimisation,
// 01 §14.2); year and month are enough to show an age in whole years.
type BirthYM struct {
	Year  int
	Month time.Month
}

var ErrBadBirthYM = errors.New("invalid birth year-month")

// ParseBirthYM parses "YYYY-MM".
func ParseBirthYM(s string) (BirthYM, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return BirthYM{}, ErrBadBirthYM
	}
	return BirthYM{t.Year(), t.Month()}, nil
}

func (b BirthYM) String() string { return fmt.Sprintf("%04d-%02d", b.Year, int(b.Month)) }

// AgeYears is the age shown to other members. Without the day of birth it counts the
// birthday as falling at the end of the birth month, so the age shown is never higher than
// the true age (at most one month late, never early).
func AgeYears(b BirthYM, now time.Time) int {
	now = now.UTC()
	age := now.Year() - b.Year
	if now.Month() <= b.Month {
		age--
	}
	return age
}

// MinimumAge is the only age at which anyone may hold a TRYST account.
const MinimumAge = 18

// Adult is TRYST's own check, applied on top of the provider's over-18 decision (which
// uses the full date of birth). Both must pass. Because AgeYears never overstates, someone
// whose 18th birthday is this month waits until next month: nobody under 18 can pass even
// if a provider is misconfigured.
func Adult(providerSaysAdult bool, b BirthYM, now time.Time) bool {
	if !providerSaysAdult || b.Year < 1900 || b.Month < 1 || b.Month > 12 {
		return false
	}
	born := time.Date(b.Year, b.Month, 1, 0, 0, 0, 0, time.UTC)
	if born.After(now) {
		return false
	}
	return AgeYears(b, now) >= MinimumAge
}
