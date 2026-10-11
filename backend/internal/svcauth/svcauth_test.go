package svcauth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSignedCalls(t *testing.T) {
	secret := bytes.Repeat([]byte{9}, 32)
	now := time.Unix(1_800_000_000, 0)
	v := &Verifier{Secrets: map[string][]byte{"profile": secret}, Now: func() time.Time { return now }}
	srv := httptest.NewServer(v.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]string{"caller": Caller(r), "body": string(b), "uri": r.URL.RequestURI()})
	})))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Caller: "profile", Secret: secret, Now: func() time.Time { return now }}
	var out map[string]string
	if err := c.Do(context.Background(), "POST", "/internal/v1/x?a=1", map[string]int{"n": 1}, &out); err != nil {
		t.Fatal(err)
	}
	if out["caller"] != "profile" || out["body"] != `{"n":1}` || out["uri"] != "/internal/v1/x?a=1" {
		t.Fatalf("out: %v", out)
	}
	for name, cl := range map[string]*Client{
		"wrong secret":   {BaseURL: srv.URL, Caller: "profile", Secret: bytes.Repeat([]byte{1}, 32), Now: c.Now},
		"unknown caller": {BaseURL: srv.URL, Caller: "mallory", Secret: secret, Now: c.Now},
		"stale":          {BaseURL: srv.URL, Caller: "profile", Secret: secret, Now: func() time.Time { return now.Add(-3 * time.Minute) }},
	} {
		if err := cl.Do(context.Background(), "GET", "/internal/v1/x", nil, nil); !IsStatus(err, 401) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A signature is bound to method, path and body: replaying it elsewhere fails.
	req, _ := http.NewRequest("POST", srv.URL+"/internal/v1/x?a=1", bytes.NewReader([]byte(`{"n":2}`)))
	ts := "1800000000"
	req.Header.Set(headerCaller, "profile")
	req.Header.Set(headerTime, ts)
	req.Header.Set(headerSig, digest(secret, "profile", ts, "POST", "/internal/v1/x?a=1", []byte(`{"n":1}`)))
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 401 {
		t.Fatalf("tampered body accepted: %d", resp.StatusCode)
	}
	if _, err := Secret("abcd"); err == nil {
		t.Fatal("short secret accepted")
	}
}

func TestRotation(t *testing.T) {
	oldS, newS := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	v := &Verifier{Secrets: map[string][]byte{"safety": newS}, Previous: map[string][]byte{"safety": oldS}}
	srv := httptest.NewServer(v.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })))
	defer srv.Close()
	for name, sec := range map[string][]byte{"new": newS, "old": oldS} {
		if err := (&Client{BaseURL: srv.URL, Caller: "safety", Secret: sec}).Do(context.Background(), "GET", "/x", nil, nil); err != nil {
			t.Errorf("%s secret refused during rotation: %v", name, err)
		}
	}
	v.Previous = nil // rotation finished
	if err := (&Client{BaseURL: srv.URL, Caller: "safety", Secret: oldS}).Do(context.Background(), "GET", "/x", nil, nil); !IsStatus(err, 401) {
		t.Fatalf("old secret accepted after rotation: %v", err)
	}
}
