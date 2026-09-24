package sql

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Migrate applies all up migrations using the same DSN as the runtime pool.
func Migrate(migrationsDir, dsn string) error {
	if migrationsDir == "" {
		return fmt.Errorf("migrations path is required")
	}
	if dsn == "" {
		return fmt.Errorf("database dsn is required")
	}
	m, err := migrate.New("file://"+migrationsDir, dsn)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
