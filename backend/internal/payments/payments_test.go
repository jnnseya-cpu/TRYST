package payments

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func sign(secret string, ts int64, payload []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(m, "%d.%s", ts, payload)
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(m.Sum(nil)))
}

func TestStripeSignature(t *testing.T) {
	now := time.Unix(1_790_000_000, 0)
	body := []byte(`{"id":"evt_1","type":"invoice.paid"}`)
	h := sign("whsec_test", now.Unix(), body)
	if err := VerifyStripeSignature(body, h, "whsec_test", now); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := VerifyStripeSignature(body, h, "whsec_other", now); err != ErrSignatureMismatch {
		t.Fatalf("wrong secret: %v", err)
	}
	if err := VerifyStripeSignature([]byte(`{"tampered":1}`), h, "whsec_test", now); err != ErrSignatureMismatch {
		t.Fatalf("tampered body: %v", err)
	}
	if err := VerifyStripeSignature(body, h, "whsec_test", now.Add(10*time.Minute)); err != ErrTimestampTolerance {
		t.Fatalf("replay: %v", err)
	}
	if err := VerifyStripeSignature(body, "garbage", "whsec_test", now); err != ErrBadSignatureHeader {
		t.Fatalf("malformed: %v", err)
	}
}

func TestDescriptorNeverNamesTryst(t *testing.T) {
	if ValidDescriptor("TRYST LONDON") || ValidDescriptor("tryst.app") || ValidDescriptor("") {
		t.Fatal("descriptor must not name TRYST")
	}
	if !ValidDescriptor("TS DIGITAL SVCS") {
		t.Fatal("neutral descriptor should pass")
	}
}

func TestDedup(t *testing.T) {
	d := NewDedup()
	if !d.First("evt_1") || d.First("evt_1") {
		t.Fatal("dedup")
	}
}
