package store

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Schema is applied on start (idempotent).
const Schema = `
CREATE TABLE IF NOT EXISTS documents (
  kind       TEXT NOT NULL,
  id         TEXT NOT NULL,
  doc        JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (kind, id)
);
CREATE TABLE IF NOT EXISTS blobs (
  id         TEXT PRIMARY KEY,
  data       BYTEA NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`

// Postgres is the production Store.
type Postgres struct {
	pool *pgxpool.Pool
	// The single-instance lock's own connection, outside the pool (Exclusive). lockMu
	// serialises it between the watchdog and Close: a pgx connection isn't safe for
	// concurrent use.
	lockMu sync.Mutex
	lock   *pgx.Conn
	closed bool
}

// OpenPostgres connects and applies the schema.
func OpenPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.Exec(ctx, Schema); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() {
	p.lockMu.Lock()
	if p.lock != nil && !p.closed {
		_ = p.lock.Close(context.Background()) // releases the single-instance lock
	}
	p.closed = true
	p.lockMu.Unlock()
	p.pool.Close()
}

// Ping checks the database is reachable (readiness).
func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

func (p *Postgres) Put(ctx context.Context, kind, id string, doc []byte) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO documents (kind, id, doc, updated_at) VALUES ($1, $2, $3, now())
		ON CONFLICT (kind, id) DO UPDATE SET doc = EXCLUDED.doc, updated_at = now()`, kind, id, doc)
	return err
}

func (p *Postgres) Get(ctx context.Context, kind, id string) ([]byte, error) {
	var doc []byte
	err := p.pool.QueryRow(ctx, `SELECT doc FROM documents WHERE kind = $1 AND id = $2`, kind, id).Scan(&doc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return doc, err
}

func (p *Postgres) Delete(ctx context.Context, kind, id string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM documents WHERE kind = $1 AND id = $2`, kind, id)
	return err
}

func (p *Postgres) All(ctx context.Context, kind string) (map[string][]byte, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, doc FROM documents WHERE kind = $1`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]byte{}
	for rows.Next() {
		var id string
		var doc []byte
		if err := rows.Scan(&id, &doc); err != nil {
			return nil, err
		}
		out[id] = doc
	}
	return out, rows.Err()
}

func (p *Postgres) PutBlob(ctx context.Context, id string, data []byte) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO blobs (id, data, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data, updated_at = now()`, id, data)
	return err
}

func (p *Postgres) GetBlob(ctx context.Context, id string) ([]byte, error) {
	var data []byte
	err := p.pool.QueryRow(ctx, `SELECT data FROM blobs WHERE id = $1`, id).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return data, err
}

func (p *Postgres) DeleteBlob(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM blobs WHERE id = $1`, id)
	return err
}

// Exclusive makes this process the only one serving the database (single writer). Services
// keep working state in memory and write through, so two copies against the same database
// would drift apart. It waits while another process holds the lock (for example the old
// copy during a deploy) and returns once it has it. If the lock's connection is ever lost,
// lost is called: the caller must stop, because another copy may now take over.
func (p *Postgres) Exclusive(ctx context.Context, name string, lost func(error)) error {
	// Its own connection, outside the pool: held for the life of the process.
	conn, err := pgx.ConnectConfig(ctx, p.pool.Config().ConnConfig.Copy())
	if err != nil {
		return err
	}
	warned := time.Time{}
	for {
		var ok bool
		if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, name).Scan(&ok); err != nil {
			_ = conn.Close(context.Background())
			return err
		}
		if ok {
			break
		}
		if time.Since(warned) > 15*time.Second {
			log.Printf("another instance holds the %s lock; waiting for it to stop", name)
			warned = time.Now()
		}
		select {
		case <-ctx.Done():
			_ = conn.Close(context.Background())
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	p.lockMu.Lock()
	p.lock = conn
	p.lockMu.Unlock()
	go func() { // #nosec G118 -- holds the lock for the life of the process, not of a request
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for range t.C {
			p.lockMu.Lock()
			if p.closed {
				p.lockMu.Unlock()
				return // Close was called: the service is stopping
			}
			c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := conn.Ping(c)
			cancel()
			p.lockMu.Unlock()
			if err != nil {
				lost(err)
				return
			}
		}
	}()
	return nil
}
