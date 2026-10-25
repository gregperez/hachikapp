// Package mailer holds the ports.Mailer adapters.
package mailer

import (
	"context"
	"log/slog"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/appconfig"
)

// Log is a Mailer that only records that an email would be sent. It never
// logs the body.
type Log struct {
	logger *slog.Logger
}

// NewLog returns the development mailer.
func NewLog(logger *slog.Logger) ports.Mailer {
	return &Log{logger: logger}
}

// Send logs the recipient and subject.
func (l *Log) Send(ctx context.Context, email domain.Email) error {
	l.logger.InfoContext(ctx, "email (log mailer)", "to", email.To, "subject", email.Subject)
	return nil
}

// FromConfig returns the SMTP mailer when SMTP_HOST is set, else the log mailer.
func FromConfig(cfg appconfig.Config, logger *slog.Logger) ports.Mailer {
	if cfg.Notifications.SMTP.Host != "" {
		return NewSMTP(cfg.Notifications.SMTP)
	}
	return NewLog(logger)
}
