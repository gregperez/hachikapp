package appconfig

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Load builds Config from defaults, then applies environment overrides.
// Database fields may also be seeded from an already-loaded YAML property bag.
// Invalid numeric/duration/boolean env values abort with a redacted field error.
func Load(seed DatabaseConfig) (Config, error) {
	cfg := Defaults()
	if seed.Host != "" {
		cfg.Database.Host = seed.Host
	}
	if seed.Port != "" {
		cfg.Database.Port = seed.Port
	}
	if seed.Name != "" {
		cfg.Database.Name = seed.Name
	}
	if seed.User != "" {
		cfg.Database.User = seed.User
	}
	if seed.Password != "" {
		cfg.Database.Password = seed.Password
	}
	if seed.SSLMode != "" {
		cfg.Database.SSLMode = seed.SSLMode
	}
	if seed.MaxConns > 0 {
		cfg.Database.MaxConns = seed.MaxConns
	}
	if seed.MigrationsPath != "" {
		cfg.Database.MigrationsPath = seed.MigrationsPath
	}

	if err := applyEnv(&cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	cfg.ApplyNonProdCORSDefaults()
	return cfg, nil
}

func applyEnv(cfg *Config) error {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.URL = v
		if err := applyDatabaseURLFields(&cfg.Database, v); err != nil {
			return err
		}
	}
	setStr(&cfg.Database.Host, "DB_HOST")
	setStr(&cfg.Database.Port, "DB_PORT")
	setStr(&cfg.Database.Name, "DB_NAME")
	setStr(&cfg.Database.User, "DB_USER")
	setStr(&cfg.Database.Password, "DB_PASSWORD")
	setStr(&cfg.Database.SSLMode, "DB_SSLMODE")
	setStr(&cfg.Database.MigrationsPath, "DB_MIGRATIONS_PATH")
	if err := setInt32(&cfg.Database.MaxConns, "DB_MAX_CONNS"); err != nil {
		return err
	}
	if err := setDuration(&cfg.Database.ConnectTimeout, "DB_CONNECT_TIMEOUT"); err != nil {
		return err
	}
	if err := setDuration(&cfg.Database.MaxConnLifetime, "DB_MAX_CONN_LIFETIME"); err != nil {
		return err
	}

	setStr(&cfg.Server.Address, "SERVER_ADDR")
	if err := setDuration(&cfg.Server.ReadHeaderTimeout, "SERVER_READ_HEADER_TIMEOUT"); err != nil {
		return err
	}
	if err := setDuration(&cfg.Server.ReadTimeout, "SERVER_READ_TIMEOUT"); err != nil {
		return err
	}
	if err := setDuration(&cfg.Server.WriteTimeout, "SERVER_WRITE_TIMEOUT"); err != nil {
		return err
	}
	if err := setDuration(&cfg.Server.IdleTimeout, "SERVER_IDLE_TIMEOUT"); err != nil {
		return err
	}
	if err := setDuration(&cfg.Server.ShutdownTimeout, "SERVER_SHUTDOWN_TIMEOUT"); err != nil {
		return err
	}
	if err := setInt(&cfg.Server.MaxHeaderBytes, "SERVER_MAX_HEADER_BYTES"); err != nil {
		return err
	}

	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		cfg.CORS.AllowedOrigins = splitCSV(v)
	}
	if v := os.Getenv("CORS_ALLOW_CREDENTIALS"); v != "" {
		b, err := parseBoolStrict(v)
		if err != nil {
			return RedactedError("cors.allowCredentials", err.Error())
		}
		cfg.CORS.AllowCredentials = b
	}

	if v := os.Getenv("SWAGGER_ENABLED"); v != "" {
		b, err := parseBoolStrict(v)
		if err != nil {
			return RedactedError("docs.swaggerEnabled", err.Error())
		}
		cfg.Docs.SwaggerEnabled = b
	}

	setStr(&cfg.Runtime.Environment, "APP_ENV")
	if cfg.Runtime.Environment == "" {
		setStr(&cfg.Runtime.Environment, "ENVIRONMENT")
	}
	setStr(&cfg.Runtime.LogLevel, "LOG_LEVEL")
	setStr(&cfg.Runtime.GinMode, "GIN_MODE")

	if v := os.Getenv("AUTH_PASSWORD_BLOCKLIST"); v != "" {
		cfg.Auth.PasswordBlocklist = splitCSV(v)
	}
	if err := setInt(&cfg.Auth.LoginAccountLimit, "AUTH_LOGIN_ACCOUNT_LIMIT"); err != nil {
		return err
	}
	if err := setInt(&cfg.Auth.LoginOriginLimit, "AUTH_LOGIN_ORIGIN_LIMIT"); err != nil {
		return err
	}
	if err := setInt(&cfg.Auth.RefreshFamilyLimit, "AUTH_REFRESH_FAMILY_LIMIT"); err != nil {
		return err
	}
	if err := setDuration(&cfg.Auth.RateLimitWindow, "AUTH_RATE_LIMIT_WINDOW"); err != nil {
		return err
	}
	if err := setInt(&cfg.Auth.RateLimitMaxEntries, "AUTH_RATE_LIMIT_MAX_ENTRIES"); err != nil {
		return err
	}
	if v := os.Getenv("AUTH_TRUSTED_PROXY_CIDRS"); v != "" {
		cfg.Auth.TrustedProxyCIDRs = splitCSV(v)
	}
	if err := setDuration(&cfg.Auth.SessionPurgeInterval, "AUTH_SESSION_PURGE_INTERVAL"); err != nil {
		return err
	}
	if err := setDuration(&cfg.Auth.SessionPurgeRetention, "AUTH_SESSION_PURGE_RETENTION"); err != nil {
		return err
	}
	if err := setInt(&cfg.Auth.SessionPurgeMaxBatches, "AUTH_SESSION_PURGE_MAX_BATCHES"); err != nil {
		return err
	}
	if err := setDuration(&cfg.Auth.AuditRetention, "AUDIT_RETENTION"); err != nil {
		return err
	}
	if err := setDuration(&cfg.Auth.AuditPurgeInterval, "AUDIT_PURGE_INTERVAL"); err != nil {
		return err
	}
	if err := setInt(&cfg.Auth.AuditPurgeMaxBatches, "AUDIT_PURGE_MAX_BATCHES"); err != nil {
		return err
	}
	if err := applyTokenKeysEnv(&cfg.Auth.TokenKeys); err != nil {
		return err
	}
	setStr(&cfg.Public.BaseURL, "PUBLIC_BASE_URL")
	setStr(&cfg.Public.VisitorPepper, "TAG_VISITOR_PEPPER")
	cfg.Public.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.Public.BaseURL), "/")
	if v := os.Getenv("NOTIFICATIONS_ENABLED"); v != "" {
		b, err := parseBoolStrict(v)
		if err != nil {
			return RedactedError("notifications.enabled", err.Error())
		}
		cfg.Notifications.Enabled = b
	}
	setStr(&cfg.Notifications.SMTP.Host, "SMTP_HOST")
	if err := setInt(&cfg.Notifications.SMTP.Port, "SMTP_PORT"); err != nil {
		return err
	}
	setStr(&cfg.Notifications.SMTP.User, "SMTP_USER")
	setStr(&cfg.Notifications.SMTP.Password, "SMTP_PASSWORD")
	setStr(&cfg.Notifications.SMTP.From, "SMTP_FROM")
	cfg.Notifications.SMTP.Host = strings.TrimSpace(cfg.Notifications.SMTP.Host)
	return applyOIDCEnv(&cfg.Auth.OIDC)
}

// applyDatabaseURLFields copies host/user/password/dbname/sslmode from DATABASE_URL
// into discrete fields so production validation sees the effective values.
func applyDatabaseURLFields(db *DatabaseConfig, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return RedactedError("database.url", "is not a valid URL")
	}
	if u.Host != "" {
		host := u.Hostname()
		port := u.Port()
		if host != "" {
			db.Host = host
		}
		if port != "" {
			db.Port = port
		}
	}
	if u.User != nil {
		if name := u.User.Username(); name != "" {
			db.User = name
		}
		if pw, ok := u.User.Password(); ok {
			db.Password = pw
		}
	}
	if path := strings.TrimPrefix(u.Path, "/"); path != "" {
		db.Name = path
	}
	q := u.Query()
	if ssl := q.Get("sslmode"); ssl != "" {
		db.SSLMode = ssl
	}
	return nil
}

func setStr(dst *string, key string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

func setInt(dst *int, key string) error {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return RedactedError(envField(key), "must be an integer")
	}
	*dst = n
	return nil
}

func setInt32(dst *int32, key string) error {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return RedactedError(envField(key), "must be an integer")
	}
	*dst = int32(n)
	return nil
}

func setDuration(dst *time.Duration, key string) error {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return RedactedError(envField(key), "must be a duration (e.g. 5s)")
	}
	*dst = d
	return nil
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseBoolStrict(v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("must be a boolean")
	}
}

func envField(envKey string) string {
	switch envKey {
	case "SMTP_PORT":
		return "notifications.smtp.port"
	case "DB_MAX_CONNS":
		return "database.maxConns"
	case "DB_CONNECT_TIMEOUT":
		return "database.connectTimeout"
	case "DB_MAX_CONN_LIFETIME":
		return "database.maxConnLifetime"
	case "SERVER_READ_HEADER_TIMEOUT":
		return "server.readHeaderTimeout"
	case "SERVER_READ_TIMEOUT":
		return "server.readTimeout"
	case "SERVER_WRITE_TIMEOUT":
		return "server.writeTimeout"
	case "SERVER_IDLE_TIMEOUT":
		return "server.idleTimeout"
	case "SERVER_SHUTDOWN_TIMEOUT":
		return "server.shutdownTimeout"
	case "SERVER_MAX_HEADER_BYTES":
		return "server.maxHeaderBytes"
	case "AUTH_LOGIN_ACCOUNT_LIMIT":
		return "auth.loginAccountLimit"
	case "AUTH_LOGIN_ORIGIN_LIMIT":
		return "auth.loginOriginLimit"
	case "AUTH_REFRESH_FAMILY_LIMIT":
		return "auth.refreshFamilyLimit"
	case "AUTH_RATE_LIMIT_WINDOW":
		return "auth.rateLimitWindow"
	case "AUTH_RATE_LIMIT_MAX_ENTRIES":
		return "auth.rateLimitMaxEntries"
	case "AUTH_SESSION_PURGE_INTERVAL":
		return "auth.sessionPurgeInterval"
	case "AUTH_SESSION_PURGE_RETENTION":
		return "auth.sessionPurgeRetention"
	case "AUTH_SESSION_PURGE_MAX_BATCHES":
		return "auth.sessionPurgeMaxBatches"
	case "AUDIT_RETENTION":
		return "auth.auditRetention"
	case "AUDIT_PURGE_INTERVAL":
		return "auth.auditPurgeInterval"
	case "AUDIT_PURGE_MAX_BATCHES":
		return "auth.auditPurgeMaxBatches"
	case "OIDC_HTTP_TIMEOUT":
		return "auth.oidc.httpTimeout"
	case "OIDC_TRANSACTION_TTL":
		return "auth.oidc.transactionTTL"
	case "OIDC_GRANT_TTL":
		return "auth.oidc.grantTTL"
	case "JWT_HMAC_OVERLAP":
		return "auth.tokenKeys.hmacOverlap"
	default:
		return strings.ToLower(envKey)
	}
}

func applyTokenKeysEnv(cfg *TokenKeysConfig) error {
	setStr(&cfg.Algorithm, "JWT_ALG")
	setStr(&cfg.AccessKID, "JWT_ACCESS_KID")
	setStr(&cfg.RefreshKID, "JWT_REFRESH_KID")
	setStr(&cfg.PreviousAccessKID, "JWT_PREVIOUS_ACCESS_KID")
	setStr(&cfg.PreviousAccessSecret, "JWT_PREVIOUS_ACCESS_SECRET")
	setStr(&cfg.PreviousRefreshKID, "JWT_PREVIOUS_REFRESH_KID")
	setStr(&cfg.PreviousRefreshSecret, "JWT_PREVIOUS_REFRESH_SECRET")
	setStr(&cfg.AccessKeyPath, "JWT_ED25519_ACCESS_KEY_PATH")
	setStr(&cfg.RefreshKeyPath, "JWT_ED25519_REFRESH_KEY_PATH")
	setStr(&cfg.PreviousAccessPubPath, "JWT_ED25519_PREVIOUS_ACCESS_PUB_PATH")
	setStr(&cfg.PreviousRefreshPubPath, "JWT_ED25519_PREVIOUS_REFRESH_PUB_PATH")
	if v := os.Getenv("JWT_HMAC_OVERLAP"); v != "" {
		b, err := parseBoolStrict(v)
		if err != nil {
			return RedactedError("auth.tokenKeys.hmacOverlap", err.Error())
		}
		cfg.HMACOverlap = b
	}
	return nil
}

func applyOIDCEnv(cfg *OIDCConfig) error {
	if v := os.Getenv("OIDC_ENABLED"); v != "" {
		b, err := parseBoolStrict(v)
		if err != nil {
			return RedactedError("auth.oidc.enabled", err.Error())
		}
		cfg.Enabled = b
	}
	if err := setDuration(&cfg.HTTPTimeout, "OIDC_HTTP_TIMEOUT"); err != nil {
		return err
	}
	if err := setDuration(&cfg.TransactionTTL, "OIDC_TRANSACTION_TTL"); err != nil {
		return err
	}
	if err := setDuration(&cfg.GrantTTL, "OIDC_GRANT_TTL"); err != nil {
		return err
	}
	setStr(&cfg.CookieName, "OIDC_COOKIE_NAME")
	setStr(&cfg.CookiePath, "OIDC_COOKIE_PATH")
	setStr(&cfg.EncryptionKey, "OIDC_ENCRYPTION_KEY")

	google := OIDCProviderConfig{
		Name:   OIDCProviderGoogle,
		Issuer: googleIssuer,
	}
	setStr(&google.Issuer, "OIDC_GOOGLE_ISSUER")
	setStr(&google.ClientID, "OIDC_GOOGLE_CLIENT_ID")
	setStr(&google.ClientSecret, "OIDC_GOOGLE_CLIENT_SECRET")
	setStr(&google.RedirectURI, "OIDC_GOOGLE_REDIRECT_URI")
	if v := os.Getenv("OIDC_GOOGLE_ALLOWED_DESTINATIONS"); v != "" {
		google.AllowedFinalDestinations = splitCSV(v)
	}
	if google.ClientID != "" || google.ClientSecret != "" || google.RedirectURI != "" || len(google.AllowedFinalDestinations) > 0 {
		replaced := false
		for i, provider := range cfg.Providers {
			if strings.EqualFold(provider.Name, OIDCProviderGoogle) {
				cfg.Providers[i] = google
				replaced = true
				break
			}
		}
		if !replaced {
			cfg.Providers = append(cfg.Providers, google)
		}
	}
	return nil
}

// RedactedError wraps validation failures without embedding secret values.
func RedactedError(field, reason string) error {
	return fmt.Errorf("%s: %s", field, reason)
}
