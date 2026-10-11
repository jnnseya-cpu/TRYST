package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
)

// Vault holds one 256-bit data key per member, each wrapped (AES-256-GCM) under a key-
// encryption key. In production the KEK is a KMS/CloudHSM key and wrapping happens in the
// HSM; here it comes from configuration. Destroying a member's data key makes everything
// sealed under it (in primaries and backups) permanently unreadable: crypto-erasure (NFR-01).
// The vault lives in its own database with its own backup policy.
type Vault struct {
	st  Store
	kek cipher.AEAD
	mu  sync.Mutex
	hot map[string]cipher.AEAD
}

var (
	ErrKEK       = errors.New("key-encryption key must be 32 bytes")
	ErrDestroyed = errors.New("data key destroyed or missing")
)

const keyKind = "datakey"

func NewVault(st Store, kek []byte) (*Vault, error) {
	if len(kek) != 32 {
		return nil, ErrKEK
	}
	b, _ := aes.NewCipher(kek)
	g, _ := cipher.NewGCM(b)
	return &Vault{st: st, kek: g, hot: map[string]cipher.AEAD{}}, nil
}

func (v *Vault) aead(ctx context.Context, ref string, create bool) (cipher.AEAD, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if a, ok := v.hot[ref]; ok {
		return a, nil
	}
	wrapped, err := v.st.Get(ctx, keyKind, ref)
	var key []byte
	switch {
	case err == nil:
		var doc struct{ W []byte }
		if err := jsonUnmarshal(wrapped, &doc); err != nil || len(doc.W) < v.kek.NonceSize() {
			return nil, ErrDestroyed
		}
		key, err = v.kek.Open(nil, doc.W[:v.kek.NonceSize()], doc.W[v.kek.NonceSize():], []byte("datakey:"+ref))
		if err != nil {
			return nil, fmt.Errorf("unwrap: %w", err)
		}
	case errors.Is(err, ErrNotFound) && create:
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		nonce := make([]byte, v.kek.NonceSize())
		_, _ = rand.Read(nonce)
		w := append(nonce, v.kek.Seal(nil, nonce, key, []byte("datakey:"+ref))...)
		doc, _ := jsonMarshal(struct{ W []byte }{w})
		if err := v.st.Put(ctx, keyKind, ref, doc); err != nil {
			return nil, err
		}
	case errors.Is(err, ErrNotFound):
		return nil, ErrDestroyed
	default:
		return nil, err
	}
	b, _ := aes.NewCipher(key)
	g, _ := cipher.NewGCM(b)
	v.hot[ref] = g
	return g, nil
}

// Seal encrypts for a member, creating their key on first use. aad binds the ciphertext to
// what it is (e.g. "photo:<id>"), so blobs cannot be swapped between rows.
func (v *Vault) Seal(ctx context.Context, ref string, pt, aad []byte) ([]byte, error) {
	a, err := v.aead(ctx, ref, true)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, a.NonceSize())
	_, _ = rand.Read(nonce)
	return append(nonce, a.Seal(nil, nonce, pt, aad)...), nil
}

// Open decrypts; it fails forever after Destroy.
func (v *Vault) Open(ctx context.Context, ref string, ct, aad []byte) ([]byte, error) {
	a, err := v.aead(ctx, ref, false)
	if err != nil {
		return nil, err
	}
	if len(ct) < a.NonceSize() {
		return nil, ErrDestroyed
	}
	return a.Open(nil, ct[:a.NonceSize()], ct[a.NonceSize():], aad)
}

// Destroy deletes the member's data key. Irreversible.
func (v *Vault) Destroy(ctx context.Context, ref string) error {
	v.mu.Lock()
	delete(v.hot, ref)
	v.mu.Unlock()
	return v.st.Delete(ctx, keyKind, ref)
}
