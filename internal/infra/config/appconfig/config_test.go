package appconfig_test

import (
	"strings"
	"testing"

	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "base_golang_test")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "secret-password-value")
	t.Setenv("DB_SSLMODE", "disable")
	t.Setenv("APP_ENV", "development")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	cfg, err := appconfig.Load(appconfig.DatabaseConfig{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	dsn, err := cfg.Database.DSN()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dsn, "sslmode=disable") {
		t.Fatalf("dsn=%s", dsn)
	}
	if !strings.Contains(dsn, "base_golang_test") {
		t.Fatalf("dsn missing db name: %s", dsn)
	}
	if len(cfg.CORS.AllowedOrigins) != 1 {
		t.Fatalf("cors=%v", cfg.CORS.AllowedOrigins)
	}
}

func TestValidateRejectsMissingPassword(t *testing.T) {
	cfg := appconfig.Defaults()
	cfg.Database.Password = ""
	cfg.Database.SSLMode = "disable"
	cfg.Runtime.Environment = "development"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "database.password") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatal("error must not include secret material")
	}
}

func TestProductionRejectsDisableSSL(t *testing.T) {
	cfg := appconfig.Defaults()
	cfg.Database.Password = "real-production-password-value"
	cfg.Database.SSLMode = "disable"
	cfg.Runtime.Environment = "production"
	cfg.CORS.AllowedOrigins = []string{"https://app.example.com"}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "database.sslMode") {
		t.Fatalf("err=%v", err)
	}
}

func TestProductionRejectsSwagger(t *testing.T) {
	cfg := appconfig.Defaults()
	cfg.Database.Password = "real-production-password-value"
	cfg.Database.SSLMode = "require"
	cfg.Runtime.Environment = "production"
	cfg.CORS.AllowedOrigins = []string{"https://app.example.com"}
	cfg.Docs.SwaggerEnabled = true
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "swaggerEnabled") {
		t.Fatalf("err=%v", err)
	}
}

func TestCORSStarWithCredentialsRejected(t *testing.T) {
	cfg := appconfig.Defaults()
	cfg.Database.Password = "secret-password-value"
	cfg.Database.SSLMode = "disable"
	cfg.Runtime.Environment = "development"
	cfg.CORS.AllowCredentials = true
	cfg.CORS.AllowedOrigins = []string{"*"}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "cors.allowedOrigins") {
		t.Fatalf("err=%v", err)
	}
}

func TestDATABASE_URLOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/base_golang_test?sslmode=disable")
	t.Setenv("APP_ENV", "test")
	cfg, err := appconfig.Load(appconfig.DatabaseConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MustDSN() != "postgres://u:p@localhost:5432/base_golang_test?sslmode=disable" {
		t.Fatalf("dsn=%s", cfg.MustDSN())
	}
	if cfg.Database.SSLMode != "disable" {
		t.Fatalf("sslMode=%s want disable from URL", cfg.Database.SSLMode)
	}
}

func TestProductionRejectsInsecureDATABASE_URL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:change-me-local@localhost:5432/db?sslmode=disable")
	t.Setenv("APP_ENV", "production")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	_, err := appconfig.Load(appconfig.DatabaseConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "change-me") || strings.Contains(err.Error(), "password=") {
		t.Fatalf("must not leak secret material: %v", err)
	}
	if !strings.Contains(err.Error(), "database.sslMode") && !strings.Contains(err.Error(), "database.password") {
		t.Fatalf("err=%v", err)
	}
}

func TestLoadRejectsInvalidIntEnv(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "base_golang_test")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "secret-password-value")
	t.Setenv("DB_SSLMODE", "disable")
	t.Setenv("APP_ENV", "development")
	t.Setenv("DB_MAX_CONNS", "not-a-number")
	_, err := appconfig.Load(appconfig.DatabaseConfig{})
	if err == nil || !strings.Contains(err.Error(), "database.maxConns") {
		t.Fatalf("err=%v", err)
	}
}

func TestLoadRejectsInvalidBoolEnv(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "base_golang_test")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "secret-password-value")
	t.Setenv("DB_SSLMODE", "disable")
	t.Setenv("APP_ENV", "development")
	t.Setenv("SWAGGER_ENABLED", "maybe")
	_, err := appconfig.Load(appconfig.DatabaseConfig{})
	if err == nil || !strings.Contains(err.Error(), "docs.swaggerEnabled") {
		t.Fatalf("err=%v", err)
	}
}
