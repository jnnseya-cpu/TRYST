package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/jnnseya-cpu/tryst/backend/internal/safetysvc"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

// TestRestoreDrill runs the real backup and restore scripts against PostgreSQL and proves
// the property the runbook relies on: an account erased after a backup comes back when the
// backup is restored, and the reapply step destroys it again, while everyone else's data
// survives intact. Needs TRYST_TEST_PG and the PostgreSQL client tools.
func TestRestoreDrill(t *testing.T) {
	base := os.Getenv("TRYST_TEST_PG")
	if base == "" {
		t.Skip("TRYST_TEST_PG not set")
	}
	for _, tool := range []string{"pg_dump", "pg_restore", "psql", "openssl", "sha256sum"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
	ctx := context.Background()
	dsn := func(name string) string {
		u, _ := url.Parse(base)
		u.Path = "/" + name
		return u.String()
	}
	admin := dsn("postgres")
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	run := fmt.Sprintf("d%x", os.Getpid())
	dbs := []string{"identity", "profile", "keys", "safety", "commerce", "ledger"}
	for _, d := range dbs {
		name := "live_" + run + "_" + d
		_, _ = conn.Exec(ctx, "drop database if exists "+name)
		if _, err := conn.Exec(ctx, "create database "+name); err != nil {
			t.Fatal(err)
		}
		t.Setenv("DATABASE_URL_"+strings.ToUpper(d), dsn(name))
	}
	key := strings.Repeat("ab", 32)
	t.Setenv("TRYST_ENV", "dev")
	t.Setenv("DATA_KEK", key)
	t.Setenv("DSR_SIGNING_KEY", key)

	startFakeSafety(t)
	t.Setenv("INTERNAL_SECRET_SAFETY", "")
	// Live system with two members' sealed data, and a safety case about each.
	live := build()
	sst, err := store.OpenPostgres(ctx, dsn("live_"+run+"_safety"))
	if err != nil {
		t.Fatal(err)
	}
	defer sst.Close()
	liveSafety, err := safetysvc.New(nil, safetysvc.StaffAuth{Env: "dev"}, nil, sst)
	if err != nil {
		t.Fatal(err)
	}
	sealed := map[string][]byte{}
	for _, acc := range []string{"acc-erased", "acc-kept"} {
		ct, err := live.vault.Seal(ctx, acc, []byte("photo of "+acc), []byte("photo:p1"))
		if err != nil {
			t.Fatal(err)
		}
		sealed[acc] = ct
		live.notifier.Notify(acc, "match", "/messages")
		liveSafety.FromSignal("th_x", "acc-other", acc, "fraud")
	}

	// Back up (encrypted, keys to their own directory).
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "backup.key")
	os.WriteFile(keyFile, []byte(hex.EncodeToString([]byte("drill-passphrase"))), 0o600)
	sh := func(env []string, name string, args ...string) string {
		cmd := exec.Command(name, args...)
		cmd.Env = append(os.Environ(), env...)
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s %v: %v\n%s\n%s", name, args, err, out.String(), errb.String())
		}
		return out.String()
	}
	backupDir := strings.TrimSpace(sh([]string{"BACKUP_DIR=" + filepath.Join(dir, "main"), "KEYS_BACKUP_DIR=" + filepath.Join(dir, "keys"), "BACKUP_KEY_FILE=" + keyFile},
		"../../../scripts/backup.sh"))
	if files, _ := filepath.Glob(filepath.Join(backupDir, "*.dump")); len(files) != 0 {
		t.Fatal("plaintext dumps left behind")
	}

	// After the backup, one member deletes their account.
	if _, _, err := live.rights.EraseAccount(ctx, "acc-erased"); err != nil {
		t.Fatal(err)
	}
	if _, err := live.vault.Open(ctx, "acc-erased", sealed["acc-erased"], []byte("photo:p1")); err == nil {
		t.Fatal("not erased live")
	}

	// Restore into scratch databases: the erased member's key is back (this is the risk).
	bin := filepath.Join(dir, "identity-svc")
	sh(nil, "go", "build", "-o", bin, ".")
	safetyBin := filepath.Join(dir, "safety-svc")
	sh(nil, "go", "build", "-o", safetyBin, "../safety-svc")
	prefix := "drill_" + run
	keysDir := filepath.Join(dir, "keys", filepath.Base(backupDir))
	out := sh([]string{"DRILL_ADMIN_DSN=" + admin, "DRILL_PREFIX=" + prefix, "DRILL_LEDGER_URL=" + dsn("live_"+run+"_ledger"),
		"TRYST_BIN=" + bin, "SAFETY_BIN=" + safetyBin, "BACKUP_KEY_FILE=" + keyFile, "KEYS_BACKUP=" + keysDir}, "../../../scripts/restore-drill.sh", backupDir)
	if strings.Count(out, `"erased_again": 1`) != 2 || strings.Count(out, `"ok": true`) != 4 {
		t.Fatalf("drill output:\n%s", out)
	}

	// The restored system no longer holds the erased member; the other member is intact.
	for _, d := range dbs[:5] {
		t.Setenv("DATABASE_URL_"+strings.ToUpper(d), dsn(prefix+"_"+d))
	}
	restored := build()
	if _, err := restored.vault.Open(ctx, "acc-erased", sealed["acc-erased"], []byte("photo:p1")); err == nil {
		t.Fatal("erased member's data readable after restore + reapply")
	}
	if pt, err := restored.vault.Open(ctx, "acc-kept", sealed["acc-kept"], []byte("photo:p1")); err != nil || string(pt) != "photo of acc-kept" {
		t.Fatalf("kept member's data lost: %v", err)
	}
	if ex, _ := restored.notifier.Export(ctx, "acc-erased"); ex != nil {
		t.Fatal("erased member's notices restored")
	}
	if ex, _ := restored.notifier.Export(ctx, "acc-kept"); ex == nil {
		t.Fatal("kept member's notices lost")
	}

	for _, d := range dbs {
		_, _ = conn.Exec(ctx, "drop database if exists live_"+run+"_"+d+" with (force)")
		_, _ = conn.Exec(ctx, "drop database if exists "+prefix+"_"+d+" with (force)")
	}
}
