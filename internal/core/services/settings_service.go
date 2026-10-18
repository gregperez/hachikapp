package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
)

type SettingsService struct {
	store ports.SettingsPort
	audit ports.SettingsAuditor
	now   func() time.Time
}

func NewSettingsService(store ports.SettingsPort, audit ports.SettingsAuditor) *SettingsService {
	if audit == nil {
		audit = ports.NoopSettingsAuditor{}
	}
	return &SettingsService{store: store, audit: audit, now: time.Now}
}

var _ core.SettingsUseCase = (*SettingsService)(nil)

func (s *SettingsService) Get(ctx context.Context) (domain.AppSettings, error) {
	if s.store == nil {
		return domain.AppSettings{}, fmt.Errorf("settings store is required")
	}
	settings, err := s.store.Get(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.DefaultAppSettings(), nil
		}
		return domain.AppSettings{}, fmt.Errorf("get settings: %w", err)
	}
	return domain.NormalizeAppSettings(settings), nil
}

func (s *SettingsService) Update(ctx context.Context, actorID string, patch domain.AppSettingsPatch) (domain.AppSettings, error) {
	if s.store == nil {
		return domain.AppSettings{}, fmt.Errorf("settings store is required")
	}
	if patch.Version <= 0 {
		return domain.AppSettings{}, fmt.Errorf("%w: version is required", domain.ErrInvalidSettings)
	}
	current, err := s.Get(ctx)
	if err != nil {
		return domain.AppSettings{}, err
	}
	if current.Version != patch.Version {
		return domain.AppSettings{}, domain.ErrStaleVersion
	}
	next := domain.NormalizeAppSettings(domain.AppSettings{
		DisplayName:   patch.DisplayName,
		LogoURL:       patch.LogoURL,
		DefaultLocale: patch.DefaultLocale,
		Version:       current.Version,
		UpdatedAt:     s.now().UTC(),
		UpdatedBy:     domain.NormalizeAuditActor(actorID),
	})
	if err := domain.ValidateAppSettings(next); err != nil {
		return domain.AppSettings{}, err
	}
	fields := domain.ChangedSettingFields(current, next)
	if len(fields) == 0 {
		return current, nil
	}
	saved, err := s.store.Update(ctx, patch.Version, next)
	if err != nil {
		return domain.AppSettings{}, err
	}
	s.audit.SettingsChange(ctx, actorID, fields)
	return domain.NormalizeAppSettings(saved), nil
}
