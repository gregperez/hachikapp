package appconfig

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Environment names.
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

// Config is the single validated runtime configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	CORS     CORSConfig
	Docs     DocsConfig
	Runtime  RuntimeConfig
	Auth     AuthConfig
	Public   PublicConfig

	Notifications NotificationsConfig
}

// DefaultVisitorPepper is the development-only pepper; production rejects it.
const DefaultVisitorPepper = "dev-only-tag-pepper"

// PublicConfig configures the anonymous public pet profile.
type PublicConfig struct {
	// BaseURL is the public web origin used to build tag links (no trailing slash).
	BaseURL string
	// VisitorPepper keys the visitor hash. Secret: never log or print it.
	VisitorPepper string
}

// NotificationsConfig configures owner email notifications.
type NotificationsConfig struct {
	// Enabled turns on email delivery (in-app notices are always created).
	Enabled bool
	SMTP    SMTPConfig
}

// SMTPConfig is the outgoing mail server. Without Host the log mailer is used.
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string // secret: never log or print it
	From     string
}

// String redacts the password so formatting the config never leaks it.
func (s SMTPConfig) String() string {
	return fmt.Sprintf("{Host:%s Port:%d User:%s Password:[redacted] From:%s}", s.Host, s.Port, s.User, s.From)
}

// GoString redacts the password for %#v.
func (s SMTPConfig) GoString() string { return s.String() }

type ServerConfig struct {
	Address           string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
}

type DatabaseConfig struct {
	// URL, when set, is used as-is for runtime and migrations.
	URL             string
	Host            string
	Port            string
	Name            string
	User            string
	Password        string
	SSLMode         string
	MaxConns        int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	MigrationsPath  string
}

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

type DocsConfig struct {
	SwaggerEnabled bool
}

type RuntimeConfig struct {
	Environment string
	LogLevel    string
	GinMode     string
}

type AuthConfig struct {
	PasswordBlocklist      []string
	LoginAccountLimit      int
	LoginOriginLimit       int
	RefreshFamilyLimit     int
	RateLimitWindow        time.Duration
	RateLimitMaxEntries    int
	TrustedProxyCIDRs      []string
	SessionPurgeInterval   time.Duration
	SessionPurgeRetention  time.Duration
	SessionPurgeMaxBatches int
	AuditRetention         time.Duration
	AuditPurgeInterval     time.Duration
	AuditPurgeMaxBatches   int
	OIDC                   OIDCConfig
	TokenKeys              TokenKeysConfig
}

// OIDCConfig is the optional federated-login configuration.
type OIDCConfig struct {
	Enabled        bool
	HTTPTimeout    time.Duration
	TransactionTTL time.Duration
	GrantTTL       time.Duration
	CookieName     string
	CookiePath     string
	EncryptionKey  string
	Providers      []OIDCProviderConfig
}

// OIDCProviderConfig is one Authorization Code + PKCE provider.
type OIDCProviderConfig struct {
	Name                     string
	Issuer                   string
	ClientID                 string
	ClientSecret             string
	RedirectURI              string
	AllowedFinalDestinations []string
}

// Defaults returns non-sensitive defaults. Secrets must come from env/file.
func Defaults() Config {
	return Config{
		Server: ServerConfig{
			Address:           ":8080",
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
			ShutdownTimeout:   15 * time.Second,
			MaxHeaderBytes:    1 << 20,
		},
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            "5432",
			Name:            "base_golang",
			User:            "postgres",
			SSLMode:         "require",
			MaxConns:        10,
			MaxConnLifetime: time.Hour,
			ConnectTimeout:  5 * time.Second,
			MigrationsPath:  "internal/resources/sql/migrations",
		},
		CORS: CORSConfig{
			AllowedOrigins:   nil,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
			AllowedHeaders:   []string{"Origin", "Content-Length", "Content-Type", "Authorization", "lang", "Accept"},
			AllowCredentials: true,
		},
		Docs: DocsConfig{SwaggerEnabled: false},
		Public: PublicConfig{
			BaseURL:       "http://localhost:3000",
			VisitorPepper: DefaultVisitorPepper,
		},
		Notifications: NotificationsConfig{
			Enabled: true,
			SMTP:    SMTPConfig{Port: 587},
		},
		Runtime: RuntimeConfig{
			Environment: EnvDevelopment,
			LogLevel:    "info",
			GinMode:     "",
		},
		Auth: AuthConfig{
			PasswordBlocklist: []string{
				"passwordpassword",
				"correct horse battery staple",
				"qwertyuiopasdfgh",
				"letmeinletmein",
			},
			LoginAccountLimit:      5,
			LoginOriginLimit:       20,
			RefreshFamilyLimit:     10,
			RateLimitWindow:        15 * time.Minute,
			RateLimitMaxEntries:    10000,
			TrustedProxyCIDRs:      nil,
			SessionPurgeInterval:   time.Hour,
			SessionPurgeRetention:  24 * time.Hour,
			SessionPurgeMaxBatches: 10,
			AuditRetention:         90 * 24 * time.Hour,
			AuditPurgeInterval:     24 * time.Hour,
			AuditPurgeMaxBatches:   10,
			OIDC: OIDCConfig{
				Enabled:        false,
				HTTPTimeout:    10 * time.Second,
				TransactionTTL: 10 * time.Minute,
				GrantTTL:       2 * time.Minute,
				CookieName:     "oidc_tx",
				CookiePath:     "/v1/auth/oidc",
			},
		},
	}
}

// DSN returns the PostgreSQL connection string shared by runtime and migrations.
func (d DatabaseConfig) DSN() (string, error) {
	if strings.TrimSpace(d.URL) != "" {
		return strings.TrimSpace(d.URL), nil
	}
	if d.Host == "" {
		return "", fmt.Errorf("database.host is required when database.url is empty")
	}
	if d.Port == "" {
		return "", fmt.Errorf("database.port is required when database.url is empty")
	}
	if d.Name == "" {
		return "", fmt.Errorf("database.name is required when database.url is empty")
	}
	if d.User == "" {
		return "", fmt.Errorf("database.user is required when database.url is empty")
	}
	ssl := d.SSLMode
	if ssl == "" {
		ssl = "require"
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(d.User, d.Password),
		Host:   d.Host + ":" + d.Port,
		Path:   "/" + d.Name,
	}
	q := u.Query()
	q.Set("sslmode", ssl)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// IsProduction reports whether the runtime environment is production.
func (c Config) IsProduction() bool {
	env := strings.ToLower(strings.TrimSpace(c.Runtime.Environment))
	return env == EnvProduction || env == "prod"
}

// TrustedProxyWarning returns the startup warning for a production process
// that trusts no proxy, or "" when there is nothing to warn about. Without
// AUTH_TRUSTED_PROXY_CIDRS the backend ignores X-Forwarded-For, so every
// public tag call and login proxied by the frontend server shares one
// client address and therefore one rate-limit bucket.
func (c Config) TrustedProxyWarning() string {
	if !c.IsProduction() || len(c.Auth.TrustedProxyCIDRs) > 0 {
		return ""
	}
	return "AUTH_TRUSTED_PROXY_CIDRS is empty: X-Forwarded-For is ignored, so the public tag and login rate limits are shared by every visitor behind the frontend server; add the frontend server's address"
}
