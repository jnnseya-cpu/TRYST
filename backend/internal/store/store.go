// Package store is TRYST's persistence layer for P1 (D-50): JSON documents and binary
// blobs in PostgreSQL, one database per domain (identity, profile, safety, commerce) so the
// identity/profile split in 03 §6 holds, and a per-member key vault for crypto-erasure.
//
// Documents are whole aggregates (an identity, a profile) written through on every change.
// The relational schema generated from 02 §4 replaces them module by module as query needs
// grow; the Store interface is the seam.
package store

import (
	"context"
	"errors"
	"sort"
	"sync"
)

var ErrNotFound = errors.New("not found")

// Store holds documents (JSON) and blobs (bytes), each addressed by kind and id.
type Store interface {
	Put(ctx context.Context, kind, id string, doc []byte) error
	Get(ctx context.Context, kind, id string) ([]byte, error)
	Delete(ctx context.Context, kind, id string) error
	All(ctx context.Context, kind string) (map[string][]byte, error)
	PutBlob(ctx context.Context, id string, data []byte) error
	GetBlob(ctx context.Context, id string) ([]byte, error)
	DeleteBlob(ctx context.Context, id string) error
}

// Memory is an in-process Store for tests and quick local runs.
type Memory struct {
	mu    sync.Mutex
	docs  map[string]map[string][]byte
	blobs map[string][]byte
}

func NewMemory() *Memory {
	return &Memory{docs: map[string]map[string][]byte{}, blobs: map[string][]byte{}}
}

func clone(b []byte) []byte { return append([]byte(nil), b...) }

func (m *Memory) Put(_ context.Context, kind, id string, doc []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.docs[kind] == nil {
		m.docs[kind] = map[string][]byte{}
	}
	m.docs[kind][id] = clone(doc)
	return nil
}

func (m *Memory) Get(_ context.Context, kind, id string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.docs[kind][id]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(d), nil
}

func (m *Memory) Delete(_ context.Context, kind, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.docs[kind], id)
	return nil
}

func (m *Memory) All(_ context.Context, kind string) (map[string][]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string][]byte{}
	for k, v := range m.docs[kind] {
		out[k] = clone(v)
	}
	return out, nil
}

func (m *Memory) PutBlob(_ context.Context, id string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blobs[id] = clone(data)
	return nil
}

func (m *Memory) GetBlob(_ context.Context, id string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.blobs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(b), nil
}

func (m *Memory) DeleteBlob(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.blobs, id)
	return nil
}

// Keys returns the sorted keys of a map (helper for deterministic loads).
func Keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
