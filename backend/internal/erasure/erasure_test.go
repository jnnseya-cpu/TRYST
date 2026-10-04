package erasure

import (
	"bytes"
	"testing"
)

func TestDestroyedKeyMakesBackupsUndecryptable(t *testing.T) {
	ks := NewKeyStore()
	if err := ks.Create("m1"); err != nil {
		t.Fatal(err)
	}
	ct, err := ks.Seal("m1", []byte("bio: private"), []byte("profile.bio_ct"))
	if err != nil {
		t.Fatal(err)
	}
	backup := append([]byte(nil), ct...) // a copy sitting in a backup
	pt, err := ks.Open("m1", ct, []byte("profile.bio_ct"))
	if err != nil || !bytes.Equal(pt, []byte("bio: private")) {
		t.Fatalf("round trip failed: %v", err)
	}
	if _, err := Erase(ks, Hooks{}, "m1"); err != nil {
		t.Fatal(err)
	}
	if _, err := ks.Open("m1", backup, []byte("profile.bio_ct")); err != ErrKeyDestroyed {
		t.Fatalf("backup must be undecryptable after erasure, got %v", err)
	}
	if err := ks.Create("m1"); err != nil {
		t.Fatal(err)
	}
	if _, err := ks.Open("m1", backup, []byte("profile.bio_ct")); err == nil {
		t.Fatal("a new key must not open old ciphertext")
	}
}

func TestAADBindsField(t *testing.T) {
	ks := NewKeyStore()
	_ = ks.Create("m")
	ct, _ := ks.Seal("m", []byte("x"), []byte("a"))
	if _, err := ks.Open("m", ct, []byte("b")); err == nil {
		t.Fatal("ciphertext must not open under a different field binding")
	}
}

func TestEraseOrderAndBanException(t *testing.T) {
	ks := NewKeyStore()
	_ = ks.Create("m")
	noop := func(string) error { return nil }
	h := Hooks{RemoveFromDiscovery: noop, RevokeDevices: noop, DeleteRows: noop, DropFeatures: noop,
		DeleteAnchorCands: noop, DeleteBioReference: noop, Banned: func(string) bool { return false }}
	j, err := Erase(ks, h, "m")
	if err != nil || j.Steps[0] != "remove_from_discovery" || j.Steps[2] != "destroy_root_key" || j.AnchorsKept {
		t.Fatalf("unexpected job %+v err %v", j, err)
	}
	_ = ks.Create("b")
	h.Banned = func(string) bool { return true }
	j, _ = Erase(ks, h, "b")
	if !j.AnchorsKept {
		t.Fatal("banned member's anchors are the declared exception and must be kept")
	}
}
