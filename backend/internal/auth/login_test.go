package auth

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

func TestEverySurfaceNeedsExactlyTwoFactors(t *testing.T) {
	for _, s := range []Surface{MobileApp, MobilePWA, Desktop, Surface("spoofed")} {
		if n := len(RequiredFactors(s)); n != 2 {
			t.Fatalf("%s requires %d factors", s, n)
		}
	}
	if f := RequiredFactors(MobilePWA); f[0] != B1 || f[1] != B2 {
		t.Fatal("mobile PWA must use two biometric factors")
	}
}

// No ordering or subset of factors short of the full required set yields a session.
func TestNoSessionWithoutBothFactors(t *testing.T) {
	all := []Factor{B1, B2, F1, F2}
	for _, s := range []Surface{MobileApp, MobilePWA, Desktop} {
		for mask := 0; mask < 1<<len(all); mask++ {
			a := NewAttempt(s, t0)
			for i, f := range all {
				if mask&(1<<i) != 0 {
					_ = a.Pass(f, t0)
				}
			}
			req := RequiredFactors(s)
			want := mask&(1<<indexOf(all, req[0])) != 0 && mask&(1<<indexOf(all, req[1])) != 0
			if a.Complete() != want {
				t.Fatalf("surface %s mask %b: complete=%v want %v", s, mask, a.Complete(), want)
			}
		}
	}
}

func indexOf(xs []Factor, f Factor) int {
	for i, x := range xs {
		if x == f {
			return i
		}
	}
	return -1
}

func TestOutOfOrderFactorRejected(t *testing.T) {
	a := NewAttempt(MobileApp, t0)
	if err := a.Pass(B2, t0); err != ErrWrongFactor {
		t.Fatalf("got %v", err)
	}
}

func TestWeakFactorsNeverAccepted(t *testing.T) {
	for _, f := range []Factor{SMSOTP, EmailOTP, MagicLink, StaffAssist} {
		if err := NewAttempt(Desktop, t0).Pass(f, t0); err != ErrNotAcceptedType {
			t.Fatalf("%s: got %v", f, err)
		}
	}
}

func TestAttemptExpires(t *testing.T) {
	a := NewAttempt(Desktop, t0)
	if err := a.Pass(F1, t0.Add(6*time.Minute)); err != ErrExpired {
		t.Fatalf("got %v", err)
	}
}

func TestBiometricEnrolmentChangeInvalidates(t *testing.T) {
	au := &Authenticator{BioEnrolmentBound: true, EnrolmentDigest: "set-1"}
	if err := au.CheckEnrolment("set-1"); err != nil {
		t.Fatal(err)
	}
	if err := au.CheckEnrolment("set-2"); err != ErrInvalidated || !au.Revoked {
		t.Fatal("enrolment change must revoke the credential")
	}
	if err := au.CheckEnrolment("set-1"); err != ErrInvalidated {
		t.Fatal("revocation is permanent")
	}
}

func TestSafetyActionsNeverGated(t *testing.T) {
	for _, a := range NeverGated {
		if StepUpRequired[a] {
			t.Fatalf("%s must never require step-up", a)
		}
	}
}

func TestSessionPolicy(t *testing.T) {
	if max, idle := SessionPolicy(Desktop); max != 12*time.Hour || idle != 15*time.Minute {
		t.Fatal("desktop policy")
	}
	if max, _ := SessionPolicy(MobileApp); max != 30*24*time.Hour {
		t.Fatal("mobile policy")
	}
}
