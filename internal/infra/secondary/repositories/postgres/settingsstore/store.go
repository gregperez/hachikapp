package settingsstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/jackc/pgx/v5"
)

type Store struct {
	db  *database.PostgresPgx
	now func() time.Time
}

func New(db *database.PostgresPgx) *Store {
	return &Store{db: db, now: time.Now}
}

var _ ports.SettingsPort = (*Store)(nil)

func (s *Store) Get(ctx context.Context) (domain.AppSettings, error) {
	if err := ctx.Err(); err != nil {
		return domain.AppSettings{}, err
	}
	row := s.db.QueryRow(ctx, `
SELECT display_name, logo_url, default_locale, version, updated_at, updated_by
FROM public.app_settings
WHERE id = 1`)
	settings, err := scanSettings(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AppSettings{}, domain.ErrNotFound
		}
		return domain.AppSettings{}, fmt.Errorf("get app settings: %w", err)
	}
	return settings, nil
}

func (s *Store) Update(ctx context.Context, expectedVersion int64, next domain.AppSettings) (domain.AppSettings, error) {
	if err := ctx.Err(); err != nil {
		return domain.AppSettings{}, err
	}
	updatedAt := next.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = s.now().UTC()
	}
	row := s.db.QueryRow(ctx, `
UPDATE public.app_settings
SET display_name = $1,
    logo_url = $2,
    default_locale = $3,
    version = version + 1,
    updated_at = $4,
    updated_by = $5
WHERE id = 1 AND version = $6
RETURNING display_name, logo_url, default_locale, version, updated_at, updated_by`,
		next.DisplayName,
		next.LogoURL,
		next.DefaultLocale,
		updatedAt,
		domain.NormalizeAuditActor(next.UpdatedBy),
		expectedVersion,
	)
	saved, err := scanSettings(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			current, getErr := s.Get(ctx)
			if getErr != nil {
				return domain.AppSettings{}, getErr
			}
			if current.Version != expectedVersion {
				return domain.AppSettings{}, domain.ErrStaleVersion
			}
			return domain.AppSettings{}, domain.ErrConflict
		}
		return domain.AppSettings{}, fmt.Errorf("update app settings: %w", err)
	}
	return saved, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSettings(row scanner) (domain.AppSettings, error) {
	var item domain.AppSettings
	if err := row.Scan(
		&item.DisplayName,
		&item.LogoURL,
		&item.DefaultLocale,
		&item.Version,
		&item.UpdatedAt,
		&item.UpdatedBy,
	); err != nil {
		return domain.AppSettings{}, err
	}
	return item, nil
}
