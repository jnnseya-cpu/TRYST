package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

// conformance runs the same checks on every Store implementation.
func conformance(t *testing.T, s Store) {
	ctx := context.Background()
	ns := fmt.Sprintf("t%d-", time.Now().UnixNano()) // isolate from other packages sharing the database
	K := func(k string) string { return ns + k }
	if _, err := s.Get(ctx, K("k"), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing doc: %v", err)
	}
	if err := s.Put(ctx, K("identity"), "a", []byte(`{"x":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, K("identity"), "a", []byte(`{"x":2}`)); err != nil {
		t.Fatal(err)
	}
	s.Put(ctx, K("identity"), "b", []byte(`{"x":3}`))
	s.Put(ctx, K("profile"), "a", []byte(`{"y":1}`))
	got, err := s.Get(ctx, K("identity"), "a")
	if err != nil || !bytes.Contains(got, []byte(`2`)) {
		t.Fatalf("upsert: %s %v", got, err)
	}
	all, _ := s.All(ctx, K("identity"))
	if len(all) != 2 {
		t.Fatalf("all: %d", len(all))
	}
	s.Delete(ctx, K("identity"), "a")
	if _, err := s.Get(ctx, K("identity"), "a"); !errors.Is(err, ErrNotFound) {
		t.Fatal("delete")
	}
	blob := []byte{0, 1, 2, 255}
	if err := s.PutBlob(ctx, K("p1"), blob); err != nil {
		t.Fatal(err)
	}
	if b, err := s.GetBlob(ctx, K("p1")); err != nil || !bytes.Equal(b, blob) {
		t.Fatalf("blob: %v %v", b, err)
	}
	s.DeleteBlob(ctx, K("p1"))
	if _, err := s.GetBlob(ctx, K("p1")); !errors.Is(err, ErrNotFound) {
		t.Fatal("blob delete")
	}
}

func TestMemory(t *testing.T) { conformance(t, NewMemory()) }

// TestPostgres runs when TRYST_TEST_PG is set (scripts/test-postgres.sh; CI service).
func TestPostgres(t *testing.T) {
	dsn := os.Getenv("TRYST_TEST_PG")
	if dsn == "" {
		t.Skip("TRYST_TEST_PG not set")
	}
	ctx := context.Background()
	p, err := OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	conformance(t, p)
}

func TestVaultCryptoErasure(t *testing.T) {
	ctx := context.Background()
	st := NewMemory()
	v, err := NewVault(st, bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := v.Seal(ctx, "member-1", []byte("photo bytes"), []byte("photo:1"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, []byte("photo bytes")) {
		t.Fatal("not encrypted")
	}
	// A fresh vault over the same store (a restart) can still open it.
	v2, _ := NewVault(st, bytes.Repeat([]byte{7}, 32))
	if pt, err := v2.Open(ctx, "member-1", ct, []byte("photo:1")); err != nil || string(pt) != "photo bytes" {
		t.Fatalf("after restart: %v", err)
	}
	if _, err := v2.Open(ctx, "member-1", ct, []byte("photo:2")); err == nil {
		t.Fatal("ciphertext moved to another row must not open")
	}
	// A different KEK cannot unwrap the key.
	v3, _ := NewVault(st, bytes.Repeat([]byte{8}, 32))
	if _, err := v3.Open(ctx, "member-1", ct, []byte("photo:1")); err == nil {
		t.Fatal("wrong KEK opened data")
	}
	// Erasure: destroy the key; nothing opens again, even with the old ciphertext (a backup).
	if err := v2.Destroy(ctx, "member-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := v2.Open(ctx, "member-1", ct, []byte("photo:1")); !errors.Is(err, ErrDestroyed) {
		t.Fatalf("after destroy: %v", err)
	}
	if _, err := NewVault(st, []byte("short")); !errors.Is(err, ErrKEK) {
		t.Fatal("short KEK accepted")
	}
}
