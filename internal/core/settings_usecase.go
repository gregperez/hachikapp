package core

import (
	"context"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// SettingsUseCase reads and updates typed application settings.
type SettingsUseCase interface {
	Get(ctx context.Context) (domain.AppSettings, error)
	Update(ctx context.Context, actorID string, patch domain.AppSettingsPatch) (domain.AppSettings, error)
}
