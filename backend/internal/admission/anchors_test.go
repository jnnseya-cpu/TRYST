package admission

import (
	"bytes"
	"testing"
)

var pepper = Pepper("test-only-pepper")

func TestAnchorsAreOneWayAndNormalised(t *testing.T) {
	a := Derive(pepper, " ab123456 ", []string{"att-1"}, "tok_1")
	b := Derive(pepper, "AB123456", []string{"att-1"}, "tok_1")
	if !bytes.Equal(a.Doc, b.Doc) {
		t.Fatal("same document must give the same anchor")
	}
	if bytes.Contains(a.Doc, []byte("AB123456")) {
		t.Fatal("anchor must not contain the document number")
	}
	if bytes.Equal(Derive(Pepper("other"), "AB123456", nil, "").Doc, a.Doc) {
		t.Fatal("anchors depend on the HSM pepper")
	}
}

func TestBannedPersonCannotRejoinOnAnyAnchor(t *testing.T) {
	var s BanStore
	banned := Derive(pepper, "AB123456", []string{"att-1"}, "tok_1")
	if !s.Ban(banned, "staff-42") {
		t.Fatal("ban with a named reviewer must succeed")
	}
	for name, attempt := range map[string]Anchors{
		"same document, new device and card": Derive(pepper, "AB123456", []string{"att-9"}, "tok_9"),
		"new document, same device":          Derive(pepper, "ZZ999999", []string{"att-1"}, "tok_9"),
		"new document, same card":            Derive(pepper, "ZZ999999", []string{"att-9"}, "tok_1"),
	} {
		if s.Admit(attempt) {
			t.Errorf("%s: should be refused", name)
		}
	}
	if !s.Admit(Derive(pepper, "ZZ999999", []string{"att-9"}, "tok_9")) {
		t.Fatal("an unrelated person must be admitted")
	}
}

func TestBanRequiresNamedHuman(t *testing.T) {
	var s BanStore
	if s.Ban(Derive(pepper, "X", nil, ""), "  ") {
		t.Fatal("permanent bans are never automated")
	}
}

func TestSuccessfulAppealLiftsAnchors(t *testing.T) {
	var s BanStore
	a := Derive(pepper, "AB123456", nil, "")
	s.Ban(a, "staff-1")
	s.Lift(a)
	if !s.Admit(a) {
		t.Fatal("anchors must be destroyed after a successful appeal")
	}
}
