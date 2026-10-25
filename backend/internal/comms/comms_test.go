package comms

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCatalogueRules(t *testing.T) {
	seen := map[string]bool{}
	cats := map[string]bool{}
	for _, c := range Categories {
		cats[c] = true
	}
	valid := map[string]bool{InApp: true, Push: true, Email: true, SMS: true, SafetyLine: true}
	for _, e := range Catalogue {
		if seen[e.Key] {
			t.Errorf("duplicate key %s", e.Key)
		}
		seen[e.Key] = true
		if !cats[e.Category] || e.Label == "" || e.Text == "" || e.Trigger == "" || len(e.Channels) == 0 {
			t.Errorf("%s incomplete", e.Key)
		}
		for _, c := range e.Channels {
			if !valid[c] {
				t.Errorf("%s: channel %q", e.Key, c) // WhatsApp is never a channel (D-29)
			}
			if c == Email {
				// An inbox may be seen by someone else: no brand, no names of the service.
				if strings.Contains(strings.ToLower(e.Subject+" "+e.Text), "tryst") || e.Subject == "" {
					t.Errorf("%s: email must have a neutral subject and never say TRYST", e.Key)
				}
			}
			if c == Push && e.Kind == "" {
				t.Errorf("%s: push only for in-app notices (content-free)", e.Key)
			}
		}
	}
}

// Every notice the code sends must be in the catalogue, so the console shows everything.
func TestEveryNoticeInCodeIsCatalogued(t *testing.T) {
	re := regexp.MustCompile(`[nN]otify\([^,()]+, "([a-z_]+)"`)
	found := map[string]bool{}
	filepath.WalkDir("..", func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, _ := os.ReadFile(p)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			found[m[1]] = true
		}
		return nil
	})
	if len(found) < 8 {
		t.Fatalf("scan found only %v", found)
	}
	for k := range found {
		if _, ok := ByKind(k); !ok {
			t.Errorf("notice kind %q is sent but not in the catalogue", k)
		}
	}
}

func TestLogKeepsNoAddressAndIsBounded(t *testing.T) {
	l := NewLog()
	l.Record("account.signup_code", Email, Sent, "smtp", "contact", "someone@example.org", false)
	s := l.Snapshot(10)
	if len(s.Recent) != 1 || strings.Contains(s.Recent[0].Recipient, "someone") || !strings.HasPrefix(s.Recent[0].Recipient, "contact·") {
		t.Fatalf("recipient: %+v", s.Recent)
	}
	if NewLog().Pseudonym("member", "acc-1") == l.Pseudonym("member", "acc-1") {
		t.Fatal("pseudonyms must change between restarts")
	}
	for i := 0; i < keepRecent+50; i++ {
		l.Record("message.new", InApp, Sent, "inbox", "member", "acc", false)
	}
	if s := l.Snapshot(0); len(s.Recent) != keepRecent || s.Totals["message.new"][Sent] != keepRecent+50 {
		t.Fatalf("recent %d totals %v", len(s.Recent), s.Totals["message.new"])
	}
}
