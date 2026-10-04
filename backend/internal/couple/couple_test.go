package couple

import (
	"testing"
	"time"
)

var now = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

func invite() *Invite {
	return &Invite{InviterProfile: "pa", InviterDeviceHash: "dev-a", InviterTierV2: true, ProfileVersion: 3, CreatedAt: now}
}

func TestCosignHappyPath(t *testing.T) {
	l, err := Cosign(invite(), "pb", "dev-b", true, 3, now.Add(time.Hour))
	if err != nil || l.ADeviceHash == l.BDeviceHash {
		t.Fatalf("err=%v link=%+v", err, l)
	}
}

func TestCosignRejections(t *testing.T) {
	cases := map[string]struct {
		prof, dev string
		v2        bool
		ver       int
		at        time.Time
		want      error
	}{
		"same device (G-NG-5)": {"pb", "dev-a", true, 3, now, ErrSameDevice},
		"self":                 {"pa", "dev-b", true, 3, now, ErrSameMember},
		"partner not V2":       {"pb", "dev-b", false, 3, now, ErrTierRequired},
		"version mismatch":     {"pb", "dev-b", true, 2, now, ErrVersionMismatch},
		"expired":              {"pb", "dev-b", true, 3, now.Add(25 * time.Hour), ErrInviteExpired},
	}
	for name, c := range cases {
		if _, err := Cosign(invite(), c.prof, c.dev, c.v2, c.ver, c.at); err != c.want {
			t.Errorf("%s: got %v want %v", name, err, c.want)
		}
	}
}

func TestInviteSingleUse(t *testing.T) {
	inv := invite()
	if _, err := Cosign(inv, "pb", "dev-b", true, 3, now); err != nil {
		t.Fatal(err)
	}
	if _, err := Cosign(inv, "pc", "dev-c", true, 3, now); err != ErrInviteUsed {
		t.Fatalf("got %v", err)
	}
}

func TestVetoModes(t *testing.T) {
	if CoupleDecision("both", Accept, Pending) != Pending {
		t.Fatal("both: one accept is not enough")
	}
	if CoupleDecision("both", Accept, Accept) != Accept {
		t.Fatal("both: two accepts")
	}
	if CoupleDecision("either", Accept, Decline) != Decline {
		t.Fatal("any decline is final")
	}
	if CoupleDecision("either", Accept, Pending) != Accept {
		t.Fatal("either: one accept suffices")
	}
}
