// Package erasure models cryptographic erasure (docs/spec/03_Backend.md §6.2-6.3; FR-038,
// NFR-01): every member's data is sealed under a per-member key; destroying that key makes
// all of their ciphertext - in primaries and in backups - permanently undecryptable.
//
// KeyStore stands in for AWS KMS + CloudHSM. Production keys never leave the HSM.
package erasure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"sync"
)

var (
	ErrKeyDestroyed = errors.New("key destroyed: data permanently undecryptable")
	ErrUnknownKey   = errors.New("unknown key")
)

// KeyStore holds per-member root keys.
type KeyStore struct {
	mu        sync.Mutex
	keys      map[string][]byte
	destroyed map[string]bool
}

func NewKeyStore() *KeyStore {
	return &KeyStore{keys: map[string][]byte{}, destroyed: map[string]bool{}}
}

// Create generates a fresh 256-bit key for a member.
func (s *KeyStore) Create(ref string) error {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[ref] = k
	return nil
}

// Destroy zeroises and forgets the key. Irreversible.
func (s *KeyStore) Destroy(ref string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if k, ok := s.keys[ref]; ok {
		for i := range k {
			k[i] = 0
		}
		delete(s.keys, ref)
	}
	s.destroyed[ref] = true
}

func (s *KeyStore) aead(ref string) (cipher.AEAD, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.destroyed[ref] {
		return nil, ErrKeyDestroyed
	}
	k, ok := s.keys[ref]
	if !ok {
		return nil, ErrUnknownKey
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Seal encrypts plaintext for a member with AES-256-GCM; aad binds the field/row identity.
func (s *KeyStore) Seal(ref string, plaintext, aad []byte) ([]byte, error) {
	g, err := s.aead(ref)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, plaintext, aad), nil
}

// Open decrypts; it fails forever once the member's key is destroyed.
func (s *KeyStore) Open(ref string, ct, aad []byte) ([]byte, error) {
	g, err := s.aead(ref)
	if err != nil {
		return nil, err
	}
	if len(ct) < g.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	return g.Open(nil, ct[:g.NonceSize()], ct[g.NonceSize():], aad)
}

// Job is the deletion orchestration result (ops.deletion_job).
type Job struct {
	SubjectRef   string
	Steps        []string
	KeyDestroyed bool
	AnchorsKept  bool // only when a permanent ban applies (declared exception, ToS 15.3)
}

// Hooks are the side effects deletion must run, in order (03 §6.3).
type Hooks struct {
	RemoveFromDiscovery func(ref string) error
	RevokeDevices       func(ref string) error
	DeleteRows          func(ref string) error
	DropFeatures        func(ref string) error
	DeleteAnchorCands   func(ref string) error
	DeleteBioReference  func(ref string) error
	Banned              func(ref string) bool
}

// Erase runs the deletion job. Discovery removal happens first so a member is never served
// after asking to leave; key destruction happens before best-effort purges.
func Erase(ks *KeyStore, h Hooks, ref string) (Job, error) {
	j := Job{SubjectRef: ref}
	run := func(name string, f func(string) error) error {
		if f == nil {
			return nil
		}
		if err := f(ref); err != nil {
			return err
		}
		j.Steps = append(j.Steps, name)
		return nil
	}
	if err := run("remove_from_discovery", h.RemoveFromDiscovery); err != nil {
		return j, err
	}
	if err := run("revoke_devices", h.RevokeDevices); err != nil {
		return j, err
	}
	ks.Destroy(ref)
	j.KeyDestroyed = true
	j.Steps = append(j.Steps, "destroy_root_key")
	for _, s := range []struct {
		n string
		f func(string) error
	}{{"delete_rows", h.DeleteRows}, {"drop_features", h.DropFeatures}, {"delete_bio_reference", h.DeleteBioReference}} {
		if err := run(s.n, s.f); err != nil {
			return j, err
		}
	}
	if h.Banned != nil && h.Banned(ref) {
		j.AnchorsKept = true
	} else if err := run("delete_anchor_candidates", h.DeleteAnchorCands); err != nil {
		return j, err
	}
	return j, nil
}
