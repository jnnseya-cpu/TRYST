package outbound

import (
	"context"
	"net"
	"net/textproto"
	"strings"
	"testing"
)

// fakeSMTP accepts one message on localhost and returns what it received.
func fakeSMTP(t *testing.T, offerTLS bool) (port string, got chan string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	got = make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		tp := textproto.NewConn(c)
		tp.PrintfLine("220 fake ESMTP")
		var b strings.Builder
		inData := false
		for {
			line, err := tp.ReadLine()
			if err != nil {
				got <- b.String()
				return
			}
			if inData {
				if line == "." {
					inData = false
					tp.PrintfLine("250 queued")
					continue
				}
				b.WriteString(line + "\n")
				continue
			}
			switch cmd := strings.ToUpper(strings.SplitN(line, " ", 2)[0]); cmd {
			case "EHLO", "HELO":
				if offerTLS {
					tp.PrintfLine("250-fake\r\n250-STARTTLS\r\n250 AUTH PLAIN")
				} else {
					tp.PrintfLine("250-fake\r\n250 AUTH PLAIN")
				}
			case "AUTH":
				b.WriteString(line + "\n")
				tp.PrintfLine("235 ok")
			case "MAIL", "RCPT":
				b.WriteString(line + "\n")
				tp.PrintfLine("250 ok")
			case "DATA":
				inData = true
				tp.PrintfLine("354 go")
			case "QUIT":
				tp.PrintfLine("221 bye")
				got <- b.String()
				return
			default:
				tp.PrintfLine("502 no")
			}
		}
	}()
	_, port, _ = net.SplitHostPort(ln.Addr().String())
	return port, got
}

func TestSMTPSendsNeutralCode(t *testing.T) {
	port, got := fakeSMTP(t, false)
	m := &SMTP{Host: "localhost", Port: port, User: "codes@example.org", Password: "pw", From: "Member services <codes@example.org>", allowPlain: true}
	if err := (Sender{Email: m}).Send("Ana@Example.com", "123456"); err != nil {
		t.Fatal(err)
	}
	msg := <-got
	for _, want := range []string{"AUTH PLAIN", "MAIL FROM:<codes@example.org>", "RCPT TO:<Ana@Example.com>", "Subject: Your verification code", "123456"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in:\n%s", want, msg)
		}
	}
	if strings.Contains(strings.ToLower(msg), "tryst") {
		t.Fatal("message names the service")
	}
}

func TestSMTPRefusesWithoutTLS(t *testing.T) {
	port, _ := fakeSMTP(t, false)
	m := &SMTP{Host: "127.0.0.1", Port: port, From: "codes@example.org"}
	if err := m.Send(context.Background(), "ana@example.com", "s", "t"); err == nil || !strings.Contains(err.Error(), "no TLS") {
		t.Fatalf("sent in the clear: %v", err)
	}
}

func TestSMTPRejectsHeaderInjection(t *testing.T) {
	m := &SMTP{Host: "localhost", Port: "1", From: "codes@example.org"}
	if err := m.Send(context.Background(), "a@example.com\r\nBcc: x@evil.example", "s", "t"); err == nil {
		t.Fatal("injected recipient accepted")
	}
}
