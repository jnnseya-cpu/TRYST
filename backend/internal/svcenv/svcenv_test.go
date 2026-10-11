package svcenv

import "testing"

func TestAdminLockModes(t *testing.T) {
	if AdminLock("reapply-erasures") != LockTry {
		t.Fatal("a writing admin command must refuse while the service runs")
	}
	for _, c := range []string{"preflight", "check-token", "verify"} {
		if AdminLock(c) != LockSkip {
			t.Fatalf("%s must run beside the live service", c)
		}
	}
}
