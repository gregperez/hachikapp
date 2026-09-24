package appconfig

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Load builds Config from defaults, then applies environment overrides.
// Database fields may also be seeded from an already-loaded YAML property bag.
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

	applyEnv(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	cfg.ApplyNonProdCORSDefaults()
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	}
	setStr(&cfg.Database.Host, "DB_HOST")
	setStr(&cfg.Database.Port, "DB_PORT")
	setStr(&cfg.Database.Name, "DB_NAME")
	setStr(&cfg.Database.User, "DB_USER")
	setStr(&cfg.Database.Password, "DB_PASSWORD")
	setStr(&cfg.Database.SSLMode, "DB_SSLMODE")
	setStr(&cfg.Database.MigrationsPath, "DB_MIGRATIONS_PATH")
	setInt32(&cfg.Database.MaxConns, "DB_MAX_CONNS")
	setDuration(&cfg.Database.ConnectTimeout, "DB_CONNECT_TIMEOUT")
	setDuration(&cfg.Database.MaxConnLifetime, "DB_MAX_CONN_LIFETIME")

	setStr(&cfg.Server.Address, "SERVER_ADDR")
	setDuration(&cfg.Server.ReadHeaderTimeout, "SERVER_READ_HEADER_TIMEOUT")
	setDuration(&cfg.Server.ReadTimeout, "SERVER_READ_TIMEOUT")
	setDuration(&cfg.Server.WriteTimeout, "SERVER_WRITE_TIMEOUT")
	setDuration(&cfg.Server.IdleTimeout, "SERVER_IDLE_TIMEOUT")
	setDuration(&cfg.Server.ShutdownTimeout, "SERVER_SHUTDOWN_TIMEOUT")
	setInt(&cfg.Server.MaxHeaderBytes, "SERVER_MAX_HEADER_BYTES")

	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		cfg.CORS.AllowedOrigins = splitCSV(v)
	}
	if v := os.Getenv("CORS_ALLOW_CREDENTIALS"); v != "" {
		cfg.CORS.AllowCredentials = parseBool(v)
	}

	if v := os.Getenv("SWAGGER_ENABLED"); v != "" {
		cfg.Docs.SwaggerEnabled = parseBool(v)
	}

	setStr(&cfg.Runtime.Environment, "APP_ENV")
	if cfg.Runtime.Environment == "" {
		setStr(&cfg.Runtime.Environment, "ENVIRONMENT")
	}
	setStr(&cfg.Runtime.LogLevel, "LOG_LEVEL")
	setStr(&cfg.Runtime.GinMode, "GIN_MODE")
}

func setStr(dst *string, key string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

func setInt(dst *int, key string) {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			*dst = n
		}
	}
}

func setInt32(dst *int32, key string) {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err == nil {
			*dst = int32(n)
		}
	}
}

func setDuration(dst *time.Duration, key string) {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			*dst = d
		}
	}
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

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// RedactedError wraps validation failures without embedding secret values.
func RedactedError(field, reason string) error {
	return fmt.Errorf("%s: %s", field, reason)
}
