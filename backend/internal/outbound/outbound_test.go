package outbound

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPostmarkAndTwilio(t *testing.T) {
	var email map[string]any
	var sms url.Values
	var smsAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		switch {
		case r.URL.Path == "/email":
			if r.Header.Get("X-Postmark-Server-Token") != "pm-token" {
				w.WriteHeader(401)
				io.WriteString(w, `{"ErrorCode":10,"Message":"bad token"}`)
				return
			}
			_ = json.Unmarshal(b, &email)
			io.WriteString(w, `{"ErrorCode":0,"Message":"OK"}`)
		case strings.HasSuffix(r.URL.Path, "/Messages.json"):
			sms, _ = url.ParseQuery(string(b))
			u, p, _ := r.BasicAuth()
			smsAuth = u + ":" + p
			if r.URL.Path != "/2010-04-01/Accounts/AC1/Messages.json" {
				w.WriteHeader(404)
				return
			}
			w.WriteHeader(201)
			io.WriteString(w, `{"sid":"SM1"}`)
		}
	}))
	defer srv.Close()
	s := Sender{
		Email: &Postmark{Token: "pm-token", From: "Member services <codes@example.org>", BaseURL: srv.URL},
		SMS:   &Twilio{AccountSID: "AC1", AuthToken: "tw-token", MessagingService: "MG1", BaseURL: srv.URL},
	}
	if err := s.Send("ana@example.com", "123456"); err != nil {
		t.Fatal(err)
	}
	if email["To"] != "ana@example.com" || !strings.Contains(email["TextBody"].(string), "123456") ||
		email["TrackOpens"] != false || email["TrackLinks"] != "None" {
		t.Fatalf("email: %v", email)
	}
	if err := s.Send("+447700900123", "654321"); err != nil {
		t.Fatal(err)
	}
	if sms.Get("To") != "+447700900123" || sms.Get("MessagingServiceSid") != "MG1" || !strings.Contains(sms.Get("Body"), "654321") || smsAuth != "AC1:tw-token" {
		t.Fatalf("sms: %v %s", sms, smsAuth)
	}
	for _, text := range []string{email["TextBody"].(string), email["Subject"].(string), sms.Get("Body")} {
		if strings.Contains(strings.ToLower(text), "tryst") || strings.Contains(strings.ToLower(text), "dating") {
			t.Fatalf("message names the service: %q", text)
		}
	}
	s.Email.Token = "wrong"
	if err := s.Send("ana@example.com", "1"); err == nil || strings.Contains(err.Error(), "ana@") {
		t.Fatalf("error leaks or missing: %v", err)
	}
	if err := (Sender{}).Send("+44", "1"); !errors.Is(err, ErrNoChannel) {
		t.Fatalf("no channel: %v", err)
	}
}
