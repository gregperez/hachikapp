package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/appconfig"
)

// sendTimeout bounds one whole SMTP conversation.
const sendTimeout = 15 * time.Second

// SMTP sends email through an SMTP server with mandatory STARTTLS
// (except on loopback hosts, used by tests).
type SMTP struct {
	cfg appconfig.SMTPConfig
	now func() time.Time
}

// NewSMTP builds the SMTP mailer.
func NewSMTP(cfg appconfig.SMTPConfig) ports.Mailer {
	return &SMTP{cfg: cfg, now: func() time.Time { return time.Now().UTC() }}
}

// requireTLS reports whether STARTTLS is mandatory for host. Only loopback
// hosts are exempt, so tests can talk to a plain fake server.
func requireTLS(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return false
	}
	return true
}

// Send delivers the email, bounded by sendTimeout and ctx.
func (s *SMTP) Send(ctx context.Context, email domain.Email) error {
	from, err := mail.ParseAddress(s.cfg.From)
	if err != nil {
		return errors.New("smtp: invalid from address")
	}
	to, err := mail.ParseAddress(email.To)
	if err != nil {
		return errors.New("smtp: invalid recipient address")
	}
	msg, err := buildMessage(s.cfg.From, email, s.now())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp: dial %s: %w", addr, err)
	}
	// Unblocks any pending read/write when the deadline or ctx expires.
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp: handshake: %w", err)
	}
	defer c.Close()

	if requireTLS(s.cfg.Host) {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("smtp: server does not offer STARTTLS")
		}
		if err := c.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("smtp: starttls: %w", err)
		}
	}
	if s.cfg.User != "" {
		if err := c.Auth(smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return fmt.Errorf("smtp: mail from: %w", err)
	}
	if err := c.Rcpt(to.Address); err != nil {
		return fmt.Errorf("smtp: rcpt to: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp: data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: end data: %w", err)
	}
	_ = c.Quit()
	return nil
}

// headerValue removes CR and LF so a value can never open a new header line.
func headerValue(v string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(v)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// buildMessage renders a multipart/alternative message (text then HTML), both
// parts UTF-8 and quoted-printable, with an RFC 2047 subject.
func buildMessage(fromHeader string, email domain.Email, now time.Time) ([]byte, error) {
	from, err := mail.ParseAddress(headerValue(fromHeader))
	if err != nil {
		return nil, errors.New("smtp: invalid from address")
	}
	to, err := mail.ParseAddress(email.To)
	if err != nil {
		return nil, errors.New("smtp: invalid recipient address")
	}
	boundary, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	idPart, err := randomHex(12)
	if err != nil {
		return nil, err
	}
	domainPart := "localhost"
	if i := strings.LastIndex(from.Address, "@"); i >= 0 && i < len(from.Address)-1 {
		domainPart = from.Address[i+1:]
	}

	var b bytes.Buffer
	hdr := func(k, v string) { b.WriteString(k + ": " + headerValue(v) + "\r\n") }
	hdr("From", from.String())
	hdr("To", to.String())
	hdr("Subject", mime.QEncoding.Encode("utf-8", headerValue(email.Subject)))
	hdr("Date", now.Format(time.RFC1123Z))
	hdr("Message-ID", "<"+idPart+"@"+domainPart+">")
	hdr("MIME-Version", "1.0")
	hdr("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	b.WriteString("\r\n")

	part := func(contentType, body string) error {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: " + contentType + "; charset=utf-8\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		qp := quotedprintable.NewWriter(&b)
		if _, err := qp.Write([]byte(body)); err != nil {
			return err
		}
		if err := qp.Close(); err != nil {
			return err
		}
		b.WriteString("\r\n")
		return nil
	}
	if err := part("text/plain", email.Text); err != nil {
		return nil, err
	}
	if err := part("text/html", email.HTML); err != nil {
		return nil, err
	}
	b.WriteString("--" + boundary + "--\r\n")
	return b.Bytes(), nil
}
