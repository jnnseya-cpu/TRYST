// Package outbound delivers sign-up codes by email (Postmark) and SMS (Twilio) (02 §3.3 V0;
// FR-119; D-55). It implements identity.Sender. Messages are neutral: they never name TRYST
// or describe the service, because a code can arrive on a shared phone or inbox. Nothing is
// stored, and neither the contact nor the code is ever logged.
package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CodeText is the neutral wording for a sign-up code.
func CodeText(code string) string {
	return "Your verification code is " + code + ". It expires in 10 minutes. If you didn't ask for it, ignore this message."
}

// Postmark sends transactional email.
type Postmark struct {
	Token   string // server token
	From    string // e.g. "Member services <codes@example.org>": neutral sender name
	BaseURL string
	HTTP    *http.Client
}

// Twilio sends SMS.
type Twilio struct {
	AccountSID       string
	AuthToken        string
	MessagingService string // MG… (preferred) or
	From             string // an alphanumeric sender id or number; neutral
	BaseURL          string
	HTTP             *http.Client
}

var ErrNoChannel = errors.New("no delivery channel configured for this contact type")

// Sender routes a code by contact type.
type Sender struct {
	Email *Postmark
	SMS   *Twilio
}

// Send implements identity.Sender.
func (s Sender) Send(contact, code string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if strings.Contains(contact, "@") {
		if s.Email == nil {
			return ErrNoChannel
		}
		return s.Email.Send(ctx, contact, "Your verification code", CodeText(code))
	}
	if s.SMS == nil {
		return ErrNoChannel
	}
	return s.SMS.Send(ctx, contact, CodeText(code))
}

func client(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// Send sends one plain-text email with tracking off (no open pixels, no rewritten links).
func (p *Postmark) Send(ctx context.Context, to, subject, text string) error {
	base := p.BaseURL
	if base == "" {
		base = "https://api.postmarkapp.com"
	}
	body, _ := json.Marshal(map[string]any{
		"From": p.From, "To": to, "Subject": subject, "TextBody": text,
		"MessageStream": "outbound", "TrackOpens": false, "TrackLinks": "None",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/email", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Postmark-Server-Token", p.Token)
	resp, err := client(p.HTTP).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		ErrorCode int    `json:"ErrorCode"`
		Message   string `json:"Message"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out)
	if resp.StatusCode != 200 || out.ErrorCode != 0 {
		return fmt.Errorf("postmark: http %d error %d", resp.StatusCode, out.ErrorCode)
	}
	return nil
}

// Send sends one SMS.
func (t *Twilio) Send(ctx context.Context, to, text string) error {
	base := t.BaseURL
	if base == "" {
		base = "https://api.twilio.com"
	}
	f := url.Values{}
	f.Set("To", to)
	f.Set("Body", text)
	if t.MessagingService != "" {
		f.Set("MessagingServiceSid", t.MessagingService)
	} else {
		f.Set("From", t.From)
	}
	u := strings.TrimRight(base, "/") + "/2010-04-01/Accounts/" + url.PathEscape(t.AccountSID) + "/Messages.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(f.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(t.AccountSID, t.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client(t.HTTP).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		var e struct {
			Code int `json:"code"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&e)
		return fmt.Errorf("twilio: http %d code %d", resp.StatusCode, e.Code)
	}
	return nil
}
