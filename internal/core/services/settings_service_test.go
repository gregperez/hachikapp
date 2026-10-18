package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/services"
)

type memSettings struct {
	current domain.AppSettings
	err     error
	stale   bool
}

func (m *memSettings) Get(context.Context) (domain.AppSettings, error) {
	if m.err != nil {
		return domain.AppSettings{}, m.err
	}
	return m.current, nil
}

func (m *memSettings) Update(_ context.Context, expectedVersion int64, next domain.AppSettings) (domain.AppSettings, error) {
	if m.stale || expectedVersion != m.current.Version {
		return domain.AppSettings{}, domain.ErrStaleVersion
	}
	next.Version = m.current.Version + 1
	m.current = next
	return next, nil
}

type memSettingsAudit struct {
	actor  string
	fields []string
}

func (m *memSettingsAudit) SettingsChange(_ context.Context, actorID string, fields []string) {
	m.actor = actorID
	m.fields = append([]string(nil), fields...)
}

func TestSettingsServiceDefaultsAndUpdate(t *testing.T) {
	t.Parallel()
	store := &memSettings{current: domain.DefaultAppSettings()}
	audit := &memSettingsAudit{}
	svc := services.NewSettingsService(store, audit)
	got, err := svc.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Base Console" {
		t.Fatalf("got=%+v", got)
	}
	updated, err := svc.Update(context.Background(), "actor-1", domain.AppSettingsPatch{
		Version: 1, DisplayName: "Acme Console", LogoURL: "https://cdn.example.com/a.png", DefaultLocale: "en",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || updated.DisplayName != "Acme Console" {
		t.Fatalf("updated=%+v", updated)
	}
	if audit.actor != "actor-1" || len(audit.fields) != 3 {
		t.Fatalf("audit=%+v", audit)
	}
	for _, field := range audit.fields {
		if field == "jwtSecret" || field == "Acme Console" {
			t.Fatalf("audit leaked %q", field)
		}
	}
}

func TestSettingsServiceRejectsUnknownLocaleAndStaleVersion(t *testing.T) {
	t.Parallel()
	store := &memSettings{current: domain.DefaultAppSettings()}
	svc := services.NewSettingsService(store, nil)
	_, err := svc.Update(context.Background(), "actor-1", domain.AppSettingsPatch{
		Version: 1, DisplayName: "Acme", DefaultLocale: "fr",
	})
	if !errors.Is(err, domain.ErrInvalidSettings) {
		t.Fatalf("locale err=%v", err)
	}
	_, err = svc.Update(context.Background(), "actor-1", domain.AppSettingsPatch{
		Version: 99, DisplayName: "Acme", DefaultLocale: "es",
	})
	if !errors.Is(err, domain.ErrStaleVersion) {
		t.Fatalf("stale err=%v", err)
	}
	_, err = svc.Update(context.Background(), "actor-1", domain.AppSettingsPatch{
		Version: 0, DisplayName: "Acme", DefaultLocale: "es",
	})
	if !errors.Is(err, domain.ErrInvalidSettings) {
		t.Fatalf("version err=%v", err)
	}
}

func TestSettingsServiceSkipsNoopUpdate(t *testing.T) {
	t.Parallel()
	store := &memSettings{current: domain.DefaultAppSettings()}
	audit := &memSettingsAudit{}
	svc := services.NewSettingsService(store, audit)
	got, err := svc.Update(context.Background(), "actor-1", domain.AppSettingsPatch{
		Version: 1, DisplayName: "Base Console", DefaultLocale: "es",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || len(audit.fields) != 0 {
		t.Fatalf("noop wrote version=%d fields=%v", got.Version, audit.fields)
	}
}
