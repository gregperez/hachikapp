package appconfig_test

import (
	"strings"
	"testing"

	"github.com/gregperez/hachikapp-backend/internal/infra/config/appconfig"
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

func TestLoadAuthenticationSecuritySettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/test?sslmode=disable")
	t.Setenv("APP_ENV", "test")
	t.Setenv("AUTH_PASSWORD_BLOCKLIST", "first blocked phrase,second blocked phrase")
	t.Setenv("AUTH_LOGIN_ACCOUNT_LIMIT", "7")
	t.Setenv("AUTH_LOGIN_ORIGIN_LIMIT", "30")
	t.Setenv("AUTH_REFRESH_FAMILY_LIMIT", "12")
	t.Setenv("AUTH_RATE_LIMIT_WINDOW", "20m")
	t.Setenv("AUTH_RATE_LIMIT_MAX_ENTRIES", "500")
	t.Setenv("AUTH_TRUSTED_PROXY_CIDRS", "10.0.0.0/8,2001:db8::/32")

	cfg, err := appconfig.Load(appconfig.DatabaseConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.LoginAccountLimit != 7 ||
		cfg.Auth.LoginOriginLimit != 30 ||
		cfg.Auth.RefreshFamilyLimit != 12 ||
		cfg.Auth.RateLimitMaxEntries != 500 {
		t.Fatalf("auth limits=%+v", cfg.Auth)
	}
	if len(cfg.Auth.PasswordBlocklist) != 2 || len(cfg.Auth.TrustedProxyCIDRs) != 2 {
		t.Fatalf("auth lists=%+v", cfg.Auth)
	}
}

func TestLoadOIDCGoogleSettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/test?sslmode=disable")
	t.Setenv("APP_ENV", "test")
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	t.Setenv("OIDC_GOOGLE_CLIENT_ID", "google-client-id")
	t.Setenv("OIDC_GOOGLE_CLIENT_SECRET", "google-client-secret-value")
	t.Setenv("OIDC_GOOGLE_REDIRECT_URI", "https://api.example.com/v1/auth/oidc/google/callback")
	t.Setenv("OIDC_GOOGLE_ALLOWED_DESTINATIONS", "https://app.example.com/auth/oidc")

	cfg, err := appconfig.Load(appconfig.DatabaseConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Auth.OIDC.Enabled {
		t.Fatal("expected OIDC enabled")
	}
	provider, ok := cfg.Auth.OIDC.Provider("google")
	if !ok {
		t.Fatal("google provider missing")
	}
	if provider.Issuer != "https://accounts.google.com" || provider.ClientID != "google-client-id" {
		t.Fatalf("provider=%+v", provider)
	}
	if len(provider.AllowedFinalDestinations) != 1 {
		t.Fatalf("destinations=%v", provider.AllowedFinalDestinations)
	}
}

func TestProductionRejectsHTTPIssuer(t *testing.T) {
	cfg := appconfig.Defaults()
	cfg.Database.Password = "real-production-password-value"
	cfg.Database.SSLMode = "require"
	cfg.Runtime.Environment = "production"
	cfg.CORS.AllowedOrigins = []string{"https://app.example.com"}
	cfg.Auth.OIDC.Enabled = true
	cfg.Auth.OIDC.EncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cfg.Auth.OIDC.Providers = []appconfig.OIDCProviderConfig{{
		Name:                     "google",
		Issuer:                   "http://accounts.google.com",
		ClientID:                 "client",
		ClientSecret:             "real-google-client-secret",
		RedirectURI:              "https://api.example.com/v1/auth/oidc/google/callback",
		AllowedFinalDestinations: []string{"https://app.example.com/auth/oidc"},
	}}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "auth.oidc.providers.issuer") {
		t.Fatalf("err=%v", err)
	}
}

func TestOIDCRejectsWildcardDestination(t *testing.T) {
	cfg := appconfig.Defaults()
	cfg.Database.Password = "secret-password-value"
	cfg.Database.SSLMode = "disable"
	cfg.Auth.OIDC.Enabled = true
	cfg.Auth.OIDC.EncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cfg.Auth.OIDC.Providers = []appconfig.OIDCProviderConfig{{
		Name:                     "google",
		Issuer:                   "https://accounts.google.com",
		ClientID:                 "client",
		ClientSecret:             "real-google-client-secret",
		RedirectURI:              "https://api.example.com/v1/auth/oidc/google/callback",
		AllowedFinalDestinations: []string{"https://*.example.com/auth"},
	}}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "wildcard") {
		t.Fatalf("err=%v", err)
	}
}

func TestTokenKeysRejectAmbiguousAlgorithm(t *testing.T) {
	cfg := appconfig.Defaults()
	cfg.Database.Password = "secret-password-value"
	cfg.Database.SSLMode = "disable"
	cfg.Auth.TokenKeys.Algorithm = "none"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "auth.tokenKeys.algorithm") {
		t.Fatalf("err=%v", err)
	}
}

func TestTokenKeysEd25519RequiresDistinctKids(t *testing.T) {
	cfg := appconfig.Defaults()
	cfg.Database.Password = "secret-password-value"
	cfg.Database.SSLMode = "disable"
	cfg.Auth.TokenKeys.Algorithm = "ed25519"
	cfg.Auth.TokenKeys.AccessKID = "same"
	cfg.Auth.TokenKeys.RefreshKID = "same"
	cfg.Auth.TokenKeys.AccessKeyPath = "/tmp/access.pem"
	cfg.Auth.TokenKeys.RefreshKeyPath = "/tmp/refresh.pem"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "auth.tokenKeys.refreshKid") {
		t.Fatalf("err=%v", err)
	}
}

func TestTokenKeysEd25519RejectsIdenticalActivePaths(t *testing.T) {
	cfg := appconfig.Defaults()
	cfg.Database.Password = "secret-password-value"
	cfg.Database.SSLMode = "disable"
	cfg.Auth.TokenKeys.Algorithm = "ed25519"
	cfg.Auth.TokenKeys.AccessKID = "access-1"
	cfg.Auth.TokenKeys.RefreshKID = "refresh-1"
	cfg.Auth.TokenKeys.AccessKeyPath = "/run/keys/shared.pem"
	cfg.Auth.TokenKeys.RefreshKeyPath = "/run/keys/shared.pem"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "auth.tokenKeys.refreshKeyPath") {
		t.Fatalf("err=%v", err)
	}
}

func TestLoadTokenKeysSettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/test?sslmode=disable")
	t.Setenv("APP_ENV", "test")
	t.Setenv("JWT_ALG", "ed25519")
	t.Setenv("JWT_ACCESS_KID", "access-1")
	t.Setenv("JWT_REFRESH_KID", "refresh-1")
	t.Setenv("JWT_ED25519_ACCESS_KEY_PATH", "/run/keys/access.pem")
	t.Setenv("JWT_ED25519_REFRESH_KEY_PATH", "/run/keys/refresh.pem")
	t.Setenv("JWT_HMAC_OVERLAP", "true")

	cfg, err := appconfig.Load(appconfig.DatabaseConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.TokenKeys.AlgorithmNorm() != appconfig.TokenAlgEd25519 {
		t.Fatalf("alg=%s", cfg.Auth.TokenKeys.Algorithm)
	}
	if !cfg.Auth.TokenKeys.HMACOverlap || cfg.Auth.TokenKeys.AccessKID != "access-1" {
		t.Fatalf("keys=%+v", cfg.Auth.TokenKeys)
	}
}

func TestValidateRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	cfg := appconfig.Defaults()
	cfg.Database.Password = "secret-password-value"
	cfg.Database.SSLMode = "disable"
	cfg.Auth.TrustedProxyCIDRs = []string{"not-a-cidr"}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "auth.trustedProxyCIDRs") {
		t.Fatalf("err=%v", err)
	}
}

func validProductionConfig() appconfig.Config {
	cfg := appconfig.Defaults()
	cfg.Database.Password = "real-production-password-value"
	cfg.Database.SSLMode = "require"
	cfg.Runtime.Environment = "production"
	cfg.CORS.AllowedOrigins = []string{"https://app.example.com"}
	cfg.Public.BaseURL = "https://app.example.com"
	cfg.Public.VisitorPepper = strings.Repeat("p", 40)
	return cfg
}

func TestPublicConfigFromEnv(t *testing.T) {
	t.Setenv("DB_PASSWORD", "secret-password-value")
	t.Setenv("DB_SSLMODE", "disable")
	t.Setenv("APP_ENV", "development")

	cfg, err := appconfig.Load(appconfig.DatabaseConfig{})
	if err != nil {
		t.Fatalf("Load defaults: %v", err)
	}
	if cfg.Public.BaseURL != "http://localhost:3000" || cfg.Public.VisitorPepper != "dev-only-tag-pepper" {
		t.Fatalf("defaults=%+v", cfg.Public)
	}

	t.Setenv("PUBLIC_BASE_URL", "https://hachik.example.com/")
	t.Setenv("TAG_VISITOR_PEPPER", "custom-pepper-value")
	cfg, err = appconfig.Load(appconfig.DatabaseConfig{})
	if err != nil {
		t.Fatalf("Load env: %v", err)
	}
	if cfg.Public.BaseURL != "https://hachik.example.com" {
		t.Fatalf("base url=%q", cfg.Public.BaseURL)
	}
	if cfg.Public.VisitorPepper != "custom-pepper-value" {
		t.Fatalf("pepper not loaded")
	}
}

func TestPublicBaseURLValidation(t *testing.T) {
	for _, bad := range []string{"", "localhost:3000", "/relative", "ftp://x.com", "http://", "https://x.com/path", "https://x.com?q=1", "https://x.com/#f"} {
		cfg := appconfig.Defaults()
		cfg.Database.Password = "pw-value"
		cfg.Public.BaseURL = bad
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "public.baseURL") {
			t.Fatalf("base=%q err=%v", bad, err)
		}
	}
	cfg := appconfig.Defaults()
	cfg.Database.Password = "pw-value"
	cfg.Public.VisitorPepper = ""
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "public.visitorPepper") {
		t.Fatalf("empty pepper err=%v", err)
	}
}

func TestProductionRejectsDefaultPepper(t *testing.T) {
	if err := validProductionConfig().Validate(); err != nil {
		t.Fatalf("valid production config: %v", err)
	}
	for _, pepper := range []string{appconfig.Defaults().Public.VisitorPepper, "short-pepper", strings.Repeat("p", 31)} {
		cfg := validProductionConfig()
		cfg.Public.VisitorPepper = pepper
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "public.visitorPepper") {
			t.Fatalf("pepper len=%d err=%v", len(pepper), err)
		}
		if strings.Contains(err.Error(), pepper) {
			t.Fatal("error must not include the pepper")
		}
	}
}

func TestProductionRequiresHTTPSPublicBaseURL(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Public.BaseURL = "http://app.example.com"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "public.baseURL") {
		t.Fatalf("err=%v", err)
	}
}

func TestTrustedProxyWarning(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Auth.TrustedProxyCIDRs = nil
	w := cfg.TrustedProxyWarning()
	if !strings.Contains(w, "AUTH_TRUSTED_PROXY_CIDRS") || !strings.Contains(w, "every visitor") {
		t.Fatalf("production without trusted proxies: warning=%q", w)
	}

	cfg.Auth.TrustedProxyCIDRs = []string{"10.0.0.0/8"}
	if w := cfg.TrustedProxyWarning(); w != "" {
		t.Fatalf("with trusted proxies: warning=%q", w)
	}

	dev := appconfig.Defaults()
	dev.Auth.TrustedProxyCIDRs = nil
	if w := dev.TrustedProxyWarning(); w != "" {
		t.Fatalf("development: warning=%q", w)
	}
}
