package domain

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	DefaultDisplayName   = "Base Console"
	DefaultLocale        = "es"
	MaxDisplayNameLength = 80
	MaxLogoURLLength     = 2048
)

// AppSettings is the singleton typed application configuration.
type AppSettings struct {
	DisplayName   string
	LogoURL       string
	DefaultLocale string
	Version       int64
	UpdatedAt     time.Time
	UpdatedBy     string
}

// PublicAppSettings is the unauthenticated branding subset.
type PublicAppSettings struct {
	DisplayName   string
	LogoURL       string
	DefaultLocale string
}

// AppSettingsPatch is the closed set of writable administrative fields.
type AppSettingsPatch struct {
	Version       int64
	DisplayName   string
	LogoURL       string
	DefaultLocale string
}

var allowedLocales = map[string]struct{}{
	"es":    {},
	"en":    {},
	"es-CL": {},
	"en-US": {},
}

var allowedSettingKeys = map[string]struct{}{
	"version":       {},
	"displayName":   {},
	"logoUrl":       {},
	"defaultLocale": {},
}

var protectedSettingKeys = map[string]struct{}{
	"jwt":           {},
	"jwtSecret":     {},
	"jwtKey":        {},
	"dsn":           {},
	"databaseUrl":   {},
	"database":      {},
	"cors":          {},
	"apiKey":        {},
	"password":      {},
	"token":         {},
	"refreshToken":  {},
	"encryptionKey": {},
	"clientSecret":  {},
	"oidc":          {},
	"timeout":       {},
}

// DefaultAppSettings returns the compiled-in branding defaults.
func DefaultAppSettings() AppSettings {
	return AppSettings{
		DisplayName:   DefaultDisplayName,
		LogoURL:       "",
		DefaultLocale: DefaultLocale,
		Version:       1,
		UpdatedBy:     AuditActorSystem,
	}
}

// PublicView strips version and actor metadata.
func PublicView(settings AppSettings) PublicAppSettings {
	return PublicAppSettings{
		DisplayName:   settings.DisplayName,
		LogoURL:       settings.LogoURL,
		DefaultLocale: settings.DefaultLocale,
	}
}

// KnownLocale reports whether locale is in the closed set.
func KnownLocale(locale string) bool {
	_, ok := allowedLocales[locale]
	return ok
}

// IsAllowedSettingKey reports whether the JSON field is writable.
func IsAllowedSettingKey(key string) bool {
	_, ok := allowedSettingKeys[key]
	return ok
}

// IsProtectedSettingKey reports operational/secret keys that must never be accepted.
func IsProtectedSettingKey(key string) bool {
	if _, ok := protectedSettingKeys[key]; ok {
		return true
	}
	lower := strings.ToLower(key)
	for needle := range protectedSettingKeys {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

// NormalizeAppSettings trims and applies defaults for empty optional fields.
func NormalizeAppSettings(settings AppSettings) AppSettings {
	settings.DisplayName = compactSettingText(settings.DisplayName)
	settings.LogoURL = strings.TrimSpace(settings.LogoURL)
	settings.DefaultLocale = strings.TrimSpace(settings.DefaultLocale)
	if settings.DisplayName == "" {
		settings.DisplayName = DefaultDisplayName
	}
	if settings.DefaultLocale == "" {
		settings.DefaultLocale = DefaultLocale
	}
	if settings.UpdatedBy == "" {
		settings.UpdatedBy = AuditActorSystem
	}
	return settings
}

// ValidateAppSettings checks display name, logo URL, and locale.
func ValidateAppSettings(settings AppSettings) error {
	name := compactSettingText(settings.DisplayName)
	if name == "" {
		return fmt.Errorf("%w: displayName is required", ErrInvalidSettings)
	}
	if utf8.RuneCountInString(name) > MaxDisplayNameLength {
		return fmt.Errorf("%w: displayName exceeds %d characters", ErrInvalidSettings, MaxDisplayNameLength)
	}
	if err := ValidateLogoURL(settings.LogoURL); err != nil {
		return err
	}
	locale := strings.TrimSpace(settings.DefaultLocale)
	if !KnownLocale(locale) {
		return fmt.Errorf("%w: defaultLocale is not supported", ErrInvalidSettings)
	}
	return nil
}

// ValidateLogoURL accepts empty, https URLs, or same-origin paths.
func ValidateLogoURL(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if utf8.RuneCountInString(value) > MaxLogoURLLength {
		return fmt.Errorf("%w: logoUrl exceeds %d characters", ErrInvalidSettings, MaxLogoURLLength)
	}
	lower := strings.ToLower(value)
	for _, needle := range []string{"javascript:", "data:", "vbscript:"} {
		if strings.HasPrefix(lower, needle) {
			return fmt.Errorf("%w: logoUrl scheme is not allowed", ErrInvalidSettings)
		}
	}
	if strings.HasPrefix(value, "/") {
		if strings.HasPrefix(value, "//") {
			return fmt.Errorf("%w: logoUrl must be a same-origin path", ErrInvalidSettings)
		}
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("%w: logoUrl must be an https URL or a path", ErrInvalidSettings)
	}
	return nil
}

// ChangedSettingFields returns allowlisted field names whose values differ.
func ChangedSettingFields(before, after AppSettings) []string {
	fields := make([]string, 0, 3)
	if before.DisplayName != after.DisplayName {
		fields = append(fields, "displayName")
	}
	if before.LogoURL != after.LogoURL {
		fields = append(fields, "logoUrl")
	}
	if before.DefaultLocale != after.DefaultLocale {
		fields = append(fields, "defaultLocale")
	}
	return fields
}

func compactSettingText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	prevSpace := false
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			continue
		}
		if unicode.IsSpace(r) {
			if prevSpace {
				continue
			}
			b.WriteRune(' ')
			prevSpace = true
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
