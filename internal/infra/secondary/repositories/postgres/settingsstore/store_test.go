package settingsstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/settingsstore"
	"github.com/WilsonSayago/base-golang/internal/test/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn, err := testdb.RequireURL()
	if err != nil {
		t.Skip(err.Error())
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return pool
}

func setupSettingsSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS public.app_settings (
			id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
			display_name varchar(80) NOT NULL,
			logo_url varchar(2048) NOT NULL DEFAULT '',
			default_locale varchar(16) NOT NULL,
			version bigint NOT NULL DEFAULT 1,
			updated_at timestamptz NOT NULL DEFAULT now(),
			updated_by varchar(64) NOT NULL DEFAULT 'system'
		)`,
		`TRUNCATE public.app_settings`,
		`INSERT INTO public.app_settings (id, display_name, logo_url, default_locale, version, updated_by)
		 VALUES (1, 'Base Console', '', 'es', 1, 'system')`,
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
}

func TestSettingsStoreGetUpdateAndConflict(t *testing.T) {
	pool := testPool(t)
	setupSettingsSchema(t, pool)
	store := settingsstore.New(database.NewPostgresPgxForTest(pool))
	ctx := context.Background()

	current, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if current.DisplayName != "Base Console" || current.Version != 1 {
		t.Fatalf("current=%+v", current)
	}

	saved, err := store.Update(ctx, 1, domain.AppSettings{
		DisplayName: "Acme", LogoURL: "/logo.svg", DefaultLocale: "en",
		UpdatedAt: time.Now().UTC(), UpdatedBy: "actor-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version != 2 || saved.DisplayName != "Acme" || saved.UpdatedBy != "actor-1" {
		t.Fatalf("saved=%+v", saved)
	}

	_, err = store.Update(ctx, 1, domain.AppSettings{
		DisplayName: "Stale", DefaultLocale: "es", UpdatedBy: "actor-2",
	})
	if !errors.Is(err, domain.ErrStaleVersion) {
		t.Fatalf("conflict err=%v", err)
	}
}
