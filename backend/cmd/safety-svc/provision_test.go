package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/staffauth"
)

// TestProvisionScript runs scripts/provision-safety-secrets.sh against a stand-in gcloud CLI.
func TestProvisionScript(t *testing.T) {
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl not installed")
	}
	store := t.TempDir()
	stub, _ := filepath.Abs("testdata")
	run := func(args ...string) (string, error) {
		cmd := exec.Command("../../../scripts/provision-safety-secrets.sh", args...)
		cmd.Env = append(os.Environ(), "ENV=prod", "FAKE_SECRETS_DIR="+store, "PATH="+stub+":"+os.Getenv("PATH"))
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	read := func(name string) string {
		b, _ := os.ReadFile(filepath.Join(store, "tryst-prod-"+name))
		return string(b)
	}
	roster := filepath.Join(t.TempDir(), "roster.txt")
	os.WriteFile(roster, []byte("# T&S team\nLead@Corp.example lead,moderator\nb@corp.example moderator\n"), 0o600)
	if out, err := run("--roster", roster); err == nil || !strings.Contains(out, "need at least 3 people") {
		t.Fatalf("small roster accepted: %s", out)
	}
	os.WriteFile(roster, []byte("lead@corp.example lead,moderator\nb@corp.example moderator\nc@corp.example moderator\n"), 0o600)
	out, err := run("--roster", roster)
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	s1, s2 := read("INTERNAL_SECRET_SAFETY"), read("INTERNAL_SECRET_PLATFORM")
	if len(s1) != 64 || len(s2) != 64 || s1 == s2 || read("INTERNAL_SECRET_SAFETY_PREVIOUS") != "none" {
		t.Fatalf("secrets: %d %d same=%v", len(s1), len(s2), s1 == s2)
	}
	if strings.Contains(out, s1) || strings.Contains(out, s2) {
		t.Fatal("a secret was printed")
	}
	if r, err := staffauth.ParseRoster(read("STAFF_ROSTER")); err != nil || len(r["lead@corp.example"]) != 2 {
		t.Fatalf("roster the service can't parse: %q %v", read("STAFF_ROSTER"), err)
	}
	// Re-running never overwrites existing secrets.
	run()
	if read("INTERNAL_SECRET_SAFETY") != s1 {
		t.Fatal("existing secret overwritten")
	}
	// Rotation keeps the old value as _PREVIOUS until finished.
	if out, err := run("--rotate"); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if read("INTERNAL_SECRET_SAFETY_PREVIOUS") != s1 || read("INTERNAL_SECRET_SAFETY") == s1 {
		t.Fatal("rotation didn't keep the previous secret")
	}
	run("--finish-rotation")
	if read("INTERNAL_SECRET_SAFETY_PREVIOUS") != "none" {
		t.Fatal("rotation not finished")
	}
	for _, bad := range []string{"not-an-email moderator", "x@corp.example admin", "x@corp.example moderator\nx@corp.example lead"} {
		os.WriteFile(roster, []byte(bad+"\na@corp.example lead\nb@corp.example moderator\n"), 0o600)
		if _, err := run("--roster", roster); err == nil {
			t.Errorf("bad roster accepted: %q", bad)
		}
	}
}
