package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/services"
	"github.com/gregperez/hachikapp-backend/internal/infra/bootstrap"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/sql"
	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("bootstrap-admin: administrator created")
}

func run() error {
	name := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_NAME"))
	email := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))
	if name == "" || email == "" {
		return fmt.Errorf("BOOTSTRAP_ADMIN_NAME and BOOTSTRAP_ADMIN_EMAIL are required")
	}

	password, err := readPassword()
	if err != nil {
		return err
	}
	defer clearString(&password)

	cfg, _, err := bootstrap.LoadRuntimeInputs(bootstrap.DefaultPropertiesFile)
	if err != nil {
		return err
	}

	dsn, err := cfg.Database.DSN()
	if err != nil {
		return err
	}
	db, err := database.OpenPool(context.Background(), cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := sql.Migrate(cfg.Database.MigrationsPath, dsn); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	users, blocklist, err := bootstrap.NewUserPersistence(db, cfg.Auth)
	if err != nil {
		return err
	}
	boot := services.NewAdminBootstrap(users, blocklist)
	if err := boot.Run(context.Background(), name, email, password); err != nil {
		if errors.Is(err, domain.ErrAdminExists) {
			return fmt.Errorf("administrator already exists")
		}
		if errors.Is(err, domain.ErrWeakPassword) {
			return fmt.Errorf("password does not meet the current policy")
		}
		return err
	}
	return nil
}

func readPassword() (string, error) {
	if v := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"); v != "" {
		_ = os.Unsetenv("BOOTSTRAP_ADMIN_PASSWORD")
		return v, nil
	}
	fmt.Fprint(os.Stderr, "Admin password: ")
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(b), nil
}

func clearString(s *string) {
	if s == nil {
		return
	}
	b := []byte(*s)
	for i := range b {
		b[i] = 0
	}
	*s = ""
}
