package store

import (
	"context"
	"errors"

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
type Postgres struct{ pool *pgxpool.Pool }

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

func (p *Postgres) Close() { p.pool.Close() }

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
