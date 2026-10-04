package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

// softAuth is a software WebAuthn authenticator for tests: it produces real attestation and
// assertion responses (ES256, "none" attestation) that the server verifies cryptographically.
type softAuth struct {
	t          *testing.T
	rpID       string
	origin     string
	attachment string // "platform" or "cross-platform"
	key        *ecdsa.PrivateKey
	credID     []byte
	userHandle []byte
	counter    uint32
	userVerify bool
}

func newSoftAuth(t *testing.T, attachment string) *softAuth {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 32)
	_, _ = rand.Read(id)
	return &softAuth{t: t, rpID: "localhost", origin: "http://localhost:3000", attachment: attachment, key: k, credID: id, userVerify: true}
}

var b64 = base64.RawURLEncoding

func (a *softAuth) flags(attested bool) byte {
	f := byte(0x01) // UP
	if a.userVerify {
		f |= 0x04 // UV
	}
	if attested {
		f |= 0x40 // AT
	}
	return f
}

func (a *softAuth) coseKey() []byte {
	x := a.key.PublicKey.X.FillBytes(make([]byte, 32))
	y := a.key.PublicKey.Y.FillBytes(make([]byte, 32))
	b, err := cbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: x, -3: y})
	if err != nil {
		a.t.Fatal(err)
	}
	return b
}

func (a *softAuth) clientData(typ, challenge string) []byte {
	b, _ := json.Marshal(map[string]any{"type": typ, "challenge": challenge, "origin": a.origin, "crossOrigin": false})
	return b
}

// options extracts publicKey.challenge, user.id and allowCredentials from server JSON.
type pkOptions struct {
	PublicKey struct {
		Challenge string `json:"challenge"`
		User      struct {
			ID string `json:"id"`
		} `json:"user"`
		AllowCredentials []struct {
			ID string `json:"id"`
		} `json:"allowCredentials"`
	} `json:"publicKey"`
}

func (a *softAuth) create(optionsJSON []byte) json.RawMessage {
	var o pkOptions
	if err := json.Unmarshal(optionsJSON, &o); err != nil {
		a.t.Fatal(err)
	}
	uh, err := b64.DecodeString(o.PublicKey.User.ID)
	if err != nil {
		a.t.Fatal(err)
	}
	a.userHandle = uh
	rp := sha256.Sum256([]byte(a.rpID))
	auth := append([]byte{}, rp[:]...)
	auth = append(auth, a.flags(true))
	auth = binary.BigEndian.AppendUint32(auth, 0)
	auth = append(auth, make([]byte, 16)...) // AAGUID
	auth = binary.BigEndian.AppendUint16(auth, uint16(len(a.credID)))
	auth = append(auth, a.credID...)
	auth = append(auth, a.coseKey()...)
	att, err := cbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": auth})
	if err != nil {
		a.t.Fatal(err)
	}
	cd := a.clientData("webauthn.create", o.PublicKey.Challenge)
	out, _ := json.Marshal(map[string]any{
		"id": b64.EncodeToString(a.credID), "rawId": b64.EncodeToString(a.credID), "type": "public-key",
		"authenticatorAttachment": a.attachment, "clientExtensionResults": map[string]any{},
		"response": map[string]any{"clientDataJSON": b64.EncodeToString(cd), "attestationObject": b64.EncodeToString(att), "transports": []string{"internal"}},
	})
	return out
}

var errNotAllowed = errors.New("credential not in allowCredentials")

// get signs an assertion. With strict=true it behaves like a browser and refuses when the
// server's allowCredentials list excludes this credential.
func (a *softAuth) get(optionsJSON []byte, strict bool) (json.RawMessage, error) {
	var o pkOptions
	if err := json.Unmarshal(optionsJSON, &o); err != nil {
		a.t.Fatal(err)
	}
	if strict && len(o.PublicKey.AllowCredentials) > 0 {
		found := false
		for _, c := range o.PublicKey.AllowCredentials {
			if c.ID == b64.EncodeToString(a.credID) {
				found = true
			}
		}
		if !found {
			return nil, errNotAllowed
		}
	}
	a.counter++
	rp := sha256.Sum256([]byte(a.rpID))
	auth := append([]byte{}, rp[:]...)
	auth = append(auth, a.flags(false))
	auth = binary.BigEndian.AppendUint32(auth, a.counter)
	cd := a.clientData("webauthn.get", o.PublicKey.Challenge)
	cdh := sha256.Sum256(cd)
	digest := sha256.Sum256(append(append([]byte{}, auth...), cdh[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		a.t.Fatal(err)
	}
	out, _ := json.Marshal(map[string]any{
		"id": b64.EncodeToString(a.credID), "rawId": b64.EncodeToString(a.credID), "type": "public-key",
		"authenticatorAttachment": a.attachment, "clientExtensionResults": map[string]any{},
		"response": map[string]any{
			"clientDataJSON": b64.EncodeToString(cd), "authenticatorData": b64.EncodeToString(auth),
			"signature": b64.EncodeToString(sig), "userHandle": b64.EncodeToString(a.userHandle),
		},
	})
	return out, nil
}
