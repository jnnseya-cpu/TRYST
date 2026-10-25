package outbound

// SMTP sends email through an ordinary mailbox provider (for example Hostinger:
// smtp.hostinger.com, port 465), so no separate email vendor is needed. TLS is required:
// implicit TLS on port 465, STARTTLS on any other port.

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Mailer sends one plain-text email.
type Mailer interface {
	Provider() string // short name for the delivery log, e.g. "hostinger smtp"
	Send(ctx context.Context, to, subject, text string) error
}

// SMTP is a Mailer over authenticated SMTP.
type SMTP struct {
	Host     string
	Port     string // "465" (implicit TLS) or "587" (STARTTLS)
	User     string
	Password string
	From     string // e.g. "Member services <codes@example.org>": neutral sender name
	// allowPlain lets tests talk to a local server without TLS. Never set in production.
	allowPlain bool
}

func header(v string) string { return strings.NewReplacer("\r", "", "\n", "").Replace(v) }

// Provider names the mailer in the delivery log ("smtp.hostinger.com", never an address).
func (s *SMTP) Provider() string { return "smtp " + s.Host }

// Send delivers the message. The contact is never logged.
func (s *SMTP) Send(ctx context.Context, to, subject, text string) error {
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("smtp: bad From: %w", err)
	}
	rcpt, err := mail.ParseAddress(to)
	if err != nil {
		return errors.New("smtp: bad recipient")
	}
	addr := net.JoinHostPort(s.Host, s.Port)
	dl, ok := ctx.Deadline()
	if !ok {
		dl = time.Now().Add(15 * time.Second)
	}
	d := &net.Dialer{Deadline: dl}
	var conn net.Conn
	if s.Port == "465" {
		conn, err = tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(dl)
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer c.Close()
	if s.Port != "465" {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return err
			}
		} else if !s.allowPlain {
			return errors.New("smtp: server offers no TLS; refusing to send")
		}
	}
	if s.User != "" {
		if err := c.Auth(smtp.PlainAuth("", s.User, s.Password, s.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(rcpt.Address); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	msg := "From: " + header(from.String()) + "\r\nTo: " + header(rcpt.Address) + "\r\nSubject: " + header(subject) +
		"\r\nDate: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: 8bit\r\n\r\n" + strings.ReplaceAll(text, "\n", "\r\n") + "\r\n"
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
