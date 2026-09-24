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
	return nil
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
	default:
		return strings.ToLower(envKey)
	}
}

// RedactedError wraps validation failures without embedding secret values.
func RedactedError(field, reason string) error {
	return fmt.Errorf("%s: %s", field, reason)
}
