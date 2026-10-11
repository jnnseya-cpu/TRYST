package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestExclusiveAllowsOneInstance(t *testing.T) {
	dsn := os.Getenv("TRYST_TEST_PG")
	if dsn == "" {
		t.Skip("TRYST_TEST_PG not set")
	}
	ctx := context.Background()
	a, err := OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	b, err := OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	name := "test-single-" + t.Name()
	if err := a.Exclusive(ctx, name, func(error) {}); err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := b.Exclusive(short, name, func(error) {}); err == nil {
		t.Fatal("a second instance got the lock")
	}
	a.Close() // the first instance stops: the lock is released with its connection
	got := make(chan error, 1)
	go func() { got <- b.Exclusive(ctx, name, func(error) {}) }()
	select {
	case err := <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the waiting instance never took over")
	}
}
