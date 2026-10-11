package assurance

import (
	"errors"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

func cert(s Scheme, days int) Cert {
	return Cert{Scheme: s, Lab: "test lab", Evidence: "report-1", ValidUntil: now.AddDate(0, 0, days)}
}

func TestFullyCertifiedProviderRunsAnywhere(t *testing.T) {
	ps := []Provider{
		{Name: "A", Up: true, Certs: []Cert{cert(PADLevel2, 200)}},
		{Name: "B", Up: true, Certs: []Cert{cert(PADLevel2, 200), cert(Injection, 200)}},
	}
	r, err := Choose(ps, Web, now)
	if err != nil || r.Provider != "B" || r.Surface != Web || r.HandOff {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestWithoutInjectionCertWebHandsOffToTheApp(t *testing.T) {
	ps := []Provider{{Name: "A", Up: true, Certs: []Cert{cert(PADLevel2, 200), cert(Injection, -1)}}} // injection expired
	r, err := Choose(ps, Web, now)
	if err != nil || r.Surface != NativeApp || !r.HandOff || !r.Attest {
		t.Fatalf("%+v %v", r, err)
	}
	if r, _ := Choose(ps, NativeApp, now); r.HandOff || !r.Attest {
		t.Fatalf("native: %+v", r)
	}
}

func TestFailsClosedWithoutLivenessCert(t *testing.T) {
	ps := []Provider{
		{Name: "A", Up: true, Certs: []Cert{cert(Injection, 200)}},                                  // no PAD
		{Name: "B", Up: true, Certs: []Cert{{Scheme: PADLevel2, ValidUntil: now.AddDate(1, 0, 0)}}}, // no evidence on file
		{Name: "C", Up: false, Certs: []Cert{cert(PADLevel2, 200), cert(Injection, 200)}},           // down
		{Name: "D", Up: true, Certs: []Cert{cert(PADLevel2, -10)}},                                  // expired
	}
	if _, err := Choose(ps, NativeApp, now); !errors.Is(err, ErrNoCertifiedProvider) {
		t.Fatalf("must fail closed: %v", err)
	}
}

func TestFailoverToSecondProvider(t *testing.T) {
	ps := []Provider{
		{Name: "A", Up: false, Certs: []Cert{cert(PADLevel2, 200), cert(Injection, 200)}},
		{Name: "B", Up: true, Certs: []Cert{cert(PADLevel2, 200)}},
	}
	if r, err := Choose(ps, NativeApp, now); err != nil || r.Provider != "B" {
		t.Fatalf("%+v %v", r, err)
	}
	if e := Expiring(ps, now, 365*24*time.Hour); len(e) != 3 {
		t.Fatalf("expiring: %v", e)
	}
}
