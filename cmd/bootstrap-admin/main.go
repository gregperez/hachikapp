package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/services"
	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/config/instance"
	"github.com/WilsonSayago/base-golang/internal/infra/config/prop"
	"github.com/WilsonSayago/base-golang/internal/infra/config/sql"
	initModules "github.com/WilsonSayago/initModules/v2"
	"golang.org/x/term"
)

const propertiesFile = "internal/resources/properties.yml"

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

	dbProp := initModules.OnceValue(prop.NewDatabaseProperty)
	if err := initModules.AddPropE(dbProp); err != nil {
		return fmt.Errorf("register database properties: %w", err)
	}
	if err := initModules.LoadProperties(
		initModules.WithFilePath(propertiesFile),
		initModules.WithFormat(initModules.YML),
	); err != nil {
		return fmt.Errorf("load properties: %w", err)
	}

	ssl := dbProp.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	cfg, err := appconfig.Load(appconfig.DatabaseConfig{
		Host:     dbProp.Host,
		Port:     dbProp.Port,
		Name:     dbProp.Name,
		User:     dbProp.User,
		Password: dbProp.Password,
		SSLMode:  ssl,
		MaxConns: dbProp.DbMaxConns,
	})
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	dsn, err := cfg.Database.DSN()
	if err != nil {
		return err
	}
	db, err := database.OpenPool(context.Background(), cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	database.SetSingleton(db)
	defer db.Close()

	if err := sql.Migrate(cfg.Database.MigrationsPath, dsn); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	boot := services.NewAdminBootstrap(instance.GetUserRepositoryInstance())
	if err := boot.Run(name, email, password); err != nil {
		if errors.Is(err, domain.ErrAdminExists) {
			return fmt.Errorf("administrator already exists")
		}
		if errors.Is(err, domain.ErrWeakPassword) {
			return fmt.Errorf("password does not meet complexity requirements")
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
