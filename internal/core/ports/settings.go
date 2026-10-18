package ports

import (
	"context"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// SettingsPort loads and updates the singleton application settings row.
type SettingsPort interface {
	Get(ctx context.Context) (domain.AppSettings, error)
	Update(ctx context.Context, expectedVersion int64, next domain.AppSettings) (domain.AppSettings, error)
}

// SettingsAuditor records administrative settings mutations without values.
type SettingsAuditor interface {
	SettingsChange(ctx context.Context, actorID string, fields []string)
}

// NoopSettingsAuditor discards settings audit events (tests).
type NoopSettingsAuditor struct{}

func (NoopSettingsAuditor) SettingsChange(context.Context, string, []string) {}
