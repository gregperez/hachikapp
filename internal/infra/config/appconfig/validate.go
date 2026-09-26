package appconfig

import (
	"fmt"
	"net/netip"
	"strings"
)

// Validate checks required fields and production safety constraints.
// Errors name the field and reason; they never include secret values.
func (c Config) Validate() error {
	if c.Server.Address == "" {
		return RedactedError("server.address", "is required")
	}
	if c.Server.ReadHeaderTimeout <= 0 {
		return RedactedError("server.readHeaderTimeout", "must be positive")
	}
	if c.Server.ReadTimeout <= 0 {
		return RedactedError("server.readTimeout", "must be positive")
	}
	if c.Server.WriteTimeout <= 0 {
		return RedactedError("server.writeTimeout", "must be positive")
	}
	if c.Server.IdleTimeout <= 0 {
		return RedactedError("server.idleTimeout", "must be positive")
	}
	if c.Server.ShutdownTimeout <= 0 {
		return RedactedError("server.shutdownTimeout", "must be positive")
	}
	if c.Server.MaxHeaderBytes <= 0 {
		return RedactedError("server.maxHeaderBytes", "must be positive")
	}

	if strings.TrimSpace(c.Database.URL) == "" {
		if c.Database.Host == "" {
			return RedactedError("database.host", "is required")
		}
		if c.Database.Port == "" {
			return RedactedError("database.port", "is required")
		}
		if c.Database.Name == "" {
			return RedactedError("database.name", "is required")
		}
		if c.Database.User == "" {
			return RedactedError("database.user", "is required")
		}
		if c.Database.Password == "" {
			return RedactedError("database.password", "is required")
		}
	}
	if c.Database.SSLMode == "" {
		return RedactedError("database.sslMode", "is required")
	}
	if c.Database.MaxConns <= 0 {
		return RedactedError("database.maxConns", "must be positive")
	}
	if c.Database.ConnectTimeout <= 0 {
		return RedactedError("database.connectTimeout", "must be positive")
	}
	if c.Database.MigrationsPath == "" {
		return RedactedError("database.migrationsPath", "is required")
	}

	if c.Runtime.Environment == "" {
		return RedactedError("runtime.environment", "is required")
	}
	if c.Runtime.LogLevel == "" {
		return RedactedError("runtime.logLevel", "is required")
	}
	if c.Auth.LoginAccountLimit <= 0 {
		return RedactedError("auth.loginAccountLimit", "must be positive")
	}
	if c.Auth.LoginOriginLimit <= 0 {
		return RedactedError("auth.loginOriginLimit", "must be positive")
	}
	if c.Auth.RefreshFamilyLimit <= 0 {
		return RedactedError("auth.refreshFamilyLimit", "must be positive")
	}
	if c.Auth.RateLimitWindow <= 0 {
		return RedactedError("auth.rateLimitWindow", "must be positive")
	}
	if c.Auth.RateLimitMaxEntries <= 0 {
		return RedactedError("auth.rateLimitMaxEntries", "must be positive")
	}
	if c.Auth.SessionPurgeInterval <= 0 {
		return RedactedError("auth.sessionPurgeInterval", "must be positive")
	}
	if c.Auth.SessionPurgeRetention < 0 {
		return RedactedError("auth.sessionPurgeRetention", "must not be negative")
	}
	if c.Auth.SessionPurgeMaxBatches <= 0 {
		return RedactedError("auth.sessionPurgeMaxBatches", "must be positive")
	}
	for _, raw := range c.Auth.TrustedProxyCIDRs {
		if _, err := netip.ParsePrefix(raw); err != nil {
			return RedactedError("auth.trustedProxyCIDRs", "contains an invalid CIDR")
		}
	}
	if err := c.Auth.OIDC.validate(c.IsProduction()); err != nil {
		return err
	}
	if err := c.Auth.TokenKeys.validate(); err != nil {
		return err
	}

	if c.CORS.AllowCredentials {
		for _, origin := range c.CORS.AllowedOrigins {
			if origin == "*" {
				return RedactedError("cors.allowedOrigins", "must not be * when credentials are allowed")
			}
		}
	}

	if c.IsProduction() {
		if err := c.validateProduction(); err != nil {
			return err
		}
	}

	if _, err := c.Database.DSN(); err != nil {
		return err
	}
	return nil
}

func (c Config) validateProduction() error {
	ssl := strings.ToLower(c.Database.SSLMode)
	if ssl == "disable" || ssl == "allow" || ssl == "prefer" {
		return RedactedError("database.sslMode", "must be require (or stronger) in production")
	}
	if len(c.CORS.AllowedOrigins) == 0 {
		return RedactedError("cors.allowedOrigins", "must be set in production")
	}
	if c.Docs.SwaggerEnabled {
		return RedactedError("docs.swaggerEnabled", "must be false in production")
	}
	if looksLikePlaceholder(c.Database.Password) {
		return RedactedError("database.password", "rejects placeholder values in production")
	}
	return nil
}

func looksLikePlaceholder(v string) bool {
	lower := strings.ToLower(v)
	return strings.Contains(lower, "change-me") ||
		strings.Contains(lower, "placeholder") ||
		strings.Contains(lower, "todo")
}

// ApplyNonProdCORSDefaults fills local origins when unset outside production.
func (c *Config) ApplyNonProdCORSDefaults() {
	if c.IsProduction() {
		return
	}
	if len(c.CORS.AllowedOrigins) == 0 {
		c.CORS.AllowedOrigins = []string{"http://localhost:3000", "http://localhost:5173"}
	}
}

// EffectiveGinMode returns the Gin mode to apply.
func (c Config) EffectiveGinMode() string {
	if c.Runtime.GinMode != "" {
		return c.Runtime.GinMode
	}
	if c.IsProduction() {
		return "release"
	}
	return "debug"
}

// MustDSN is a helper for tests; panics only on programmer error after Validate.
func (c Config) MustDSN() string {
	dsn, err := c.Database.DSN()
	if err != nil {
		panic(fmt.Sprintf("dsn after validate: %v", err))
	}
	return dsn
}
