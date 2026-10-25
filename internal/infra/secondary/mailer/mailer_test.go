package mailer

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/appconfig"
)

var fixedNow = time.Date(2026, 10, 24, 15, 4, 5, 0, time.UTC)

func parse(t *testing.T, raw []byte) *mail.Message {
	t.Helper()
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadMessage: %v\n%s", err, raw)
	}
	return m
}

func TestBuildMessageMultipartUTF8(t *testing.T) {
	raw, err := buildMessage("Hachikapp <no-reply@example.com>", domain.Email{
		To:      "dueña@example.com",
		Subject: "Alguien escaneó el Hachikapp Tag de Mía",
		Text:    "Hola ñandú\nlínea 2",
		HTML:    "<p>Hola ñandú</p>",
	}, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	m := parse(t, raw)

	dec := new(mime.WordDecoder)
	subject, err := dec.DecodeHeader(m.Header.Get("Subject"))
	if err != nil || subject != "Alguien escaneó el Hachikapp Tag de Mía" {
		t.Fatalf("subject=%q err=%v", subject, err)
	}
	if !strings.Contains(m.Header.Get("Subject"), "=?utf-8?q?") {
		t.Fatalf("subject not RFC 2047 encoded: %q", m.Header.Get("Subject"))
	}
	from, err := m.Header.AddressList("From")
	if err != nil || len(from) != 1 || from[0].Address != "no-reply@example.com" || from[0].Name != "Hachikapp" {
		t.Fatalf("from=%v err=%v", from, err)
	}
	to, err := m.Header.AddressList("To")
	if err != nil || len(to) != 1 || to[0].Address != "dueña@example.com" {
		t.Fatalf("to=%v err=%v", to, err)
	}
	if m.Header.Get("MIME-Version") != "1.0" || m.Header.Get("Date") == "" || !strings.HasPrefix(m.Header.Get("Message-ID"), "<") {
		t.Fatalf("headers=%v", m.Header)
	}
	mt, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/alternative" {
		t.Fatalf("content-type=%q err=%v", mt, err)
	}
	mr := multipart.NewReader(m.Body, params["boundary"])
	var types, bodies []string
	for {
		p, err := mr.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		pt, pp, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if pp["charset"] != "utf-8" || p.Header.Get("Content-Transfer-Encoding") != "quoted-printable" {
			t.Fatalf("part headers=%v", p.Header)
		}
		b, _ := io.ReadAll(quotedprintable.NewReader(p))
		types = append(types, pt)
		bodies = append(bodies, string(b))
	}
	if len(types) != 2 || types[0] != "text/plain" || types[1] != "text/html" {
		t.Fatalf("types=%v", types)
	}
	if !strings.Contains(bodies[0], "Hola ñandú") || !strings.Contains(bodies[1], "<p>Hola ñandú</p>") {
		t.Fatalf("bodies=%q", bodies)
	}
}

func TestBuildMessageHeaderInjection(t *testing.T) {
	raw, err := buildMessage("Hachikapp <no-reply@example.com>", domain.Email{
		To:      "owner@example.com",
		Subject: "Alguien escaneó a Rex\r\nBcc: x@y.com",
		Text:    "t", HTML: "h",
	}, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	head, _, _ := strings.Cut(string(raw), "\r\n\r\n")
	for _, line := range strings.Split(head, "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), "bcc:") {
			t.Fatalf("injected header: %q", line)
		}
	}
	m := parse(t, raw)
	if m.Header.Get("Bcc") != "" {
		t.Fatal("Bcc header present")
	}
	// A recipient trying the same trick is rejected outright.
	if _, err := buildMessage("a@example.com", domain.Email{To: "a@example.com\r\nBcc: x@y.com", Subject: "s"}, fixedNow); err == nil {
		t.Fatal("expected error for injected recipient")
	}
}

func TestRequireTLS(t *testing.T) {
	for host, want := range map[string]bool{
		"smtp.example.com": true,
		"localhost":        false,
		"LocalHost":        false,
		"127.0.0.1":        false,
		"::1":              false,
		"127.0.0.2":        true,
		"mail.localhost":   true,
		"10.0.0.5":         true,
	} {
		if got := requireTLS(host); got != want {
			t.Errorf("requireTLS(%q)=%v want %v", host, got, want)
		}
	}
}

type fakeSMTP struct {
	ln   net.Listener
	mu   sync.Mutex
	cmds []string
	data string
	auth string
}

func newFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{ln: ln}
	t.Cleanup(func() { ln.Close() })
	go f.serve()
	return f
}

func (f *fakeSMTP) serve() {
	conn, err := f.ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := func(s string) { conn.Write([]byte(s + "\r\n")) }
	w("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		f.mu.Lock()
		f.cmds = append(f.cmds, line)
		f.mu.Unlock()
		up := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(up, "EHLO"):
			w("250-fake")
			w("250 AUTH PLAIN")
		case strings.HasPrefix(up, "AUTH"):
			f.mu.Lock()
			f.auth = line
			f.mu.Unlock()
			w("235 ok")
		case strings.HasPrefix(up, "MAIL"), strings.HasPrefix(up, "RCPT"):
			w("250 ok")
		case up == "DATA":
			w("354 go")
			var sb strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				sb.WriteString(l)
			}
			f.mu.Lock()
			f.data = sb.String()
			f.mu.Unlock()
			w("250 queued")
		case up == "QUIT":
			w("221 bye")
			return
		default:
			w("250 ok")
		}
	}
}

func (f *fakeSMTP) port() int { return f.ln.Addr().(*net.TCPAddr).Port }

func TestSMTPSendAgainstFakeServer(t *testing.T) {
	f := newFakeSMTP(t)
	m := NewSMTP(appconfig.SMTPConfig{Host: "127.0.0.1", Port: f.port(), From: "Hachikapp <no-reply@example.com>"})
	err := m.Send(context.Background(), domain.Email{
		To: "owner@example.com", Subject: "Alguien escaneó el Hachikapp Tag de Rex",
		Text: "hola\n.\npunto solo", HTML: "<p>hola</p>",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	joined := strings.Join(f.cmds, "|")
	for _, want := range []string{"EHLO", "MAIL FROM:<no-reply@example.com>", "RCPT TO:<owner@example.com>", "DATA", "QUIT"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
	if f.auth != "" {
		t.Errorf("AUTH sent without a user: %q", f.auth)
	}
	msg := parse(t, []byte(f.data))
	if msg.Header.Get("To") != "<owner@example.com>" && !strings.Contains(msg.Header.Get("To"), "owner@example.com") {
		t.Fatalf("To=%q", msg.Header.Get("To"))
	}
	if !strings.HasPrefix(msg.Header.Get("Content-Type"), "multipart/alternative") {
		t.Fatalf("content-type=%q", msg.Header.Get("Content-Type"))
	}
}

func TestSMTPSendUsesAuthOnlyWithUser(t *testing.T) {
	f := newFakeSMTP(t)
	m := NewSMTP(appconfig.SMTPConfig{Host: "127.0.0.1", Port: f.port(), User: "mailer", Password: "pw", From: "no-reply@example.com"})
	if err := m.Send(context.Background(), domain.Email{To: "o@example.com", Subject: "s", Text: "t", HTML: "h"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !strings.HasPrefix(f.auth, "AUTH PLAIN") {
		t.Fatalf("auth=%q", f.auth)
	}
}

func TestSMTPSendTimeoutError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() { // accepts and stays silent
		c, err := ln.Accept()
		if err == nil {
			defer c.Close()
			time.Sleep(2 * time.Second)
		}
	}()
	m := NewSMTP(appconfig.SMTPConfig{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, From: "a@example.com", Password: "topsecret"})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = m.Send(ctx, domain.Email{To: "o@example.com", Subject: "s"})
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("err=%v elapsed=%v", err, time.Since(start))
	}
	if strings.Contains(err.Error(), "topsecret") {
		t.Fatal("error leaks password")
	}
}

func TestLogMailerDoesNotLogBody(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	err := NewLog(logger).Send(context.Background(), domain.Email{
		To: "owner@example.com", Subject: "Asunto visible", Text: "CUERPO-SECRETO-TEXTO", HTML: "<p>CUERPO-SECRETO-HTML</p>",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "email (log mailer)") || !strings.Contains(out, "owner@example.com") || !strings.Contains(out, "Asunto visible") {
		t.Fatalf("log=%q", out)
	}
	if strings.Contains(out, "CUERPO-SECRETO") {
		t.Fatalf("body logged: %q", out)
	}
}

func TestFromConfigSelectsAdapter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := appconfig.Defaults()
	if _, ok := FromConfig(cfg, logger).(*Log); !ok {
		t.Fatal("expected log mailer without SMTP_HOST")
	}
	cfg.Notifications.SMTP = appconfig.SMTPConfig{Host: "smtp.example.com", Port: 587, From: "a@example.com"}
	if _, ok := FromConfig(cfg, logger).(*SMTP); !ok {
		t.Fatal("expected SMTP mailer with SMTP_HOST")
	}
}
