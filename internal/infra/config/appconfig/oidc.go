package appconfig

import (
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"strings"
)

const (
	OIDCProviderGoogle = "google"
	googleIssuer       = "https://accounts.google.com"
)

// Provider returns the named provider when present.
func (c OIDCConfig) Provider(name string) (OIDCProviderConfig, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, provider := range c.Providers {
		if strings.ToLower(strings.TrimSpace(provider.Name)) == want {
			return provider, true
		}
	}
	return OIDCProviderConfig{}, false
}

// EncryptionKeyBytes returns the 32-byte AES key. The raw secret is never returned.
func (c OIDCConfig) EncryptionKeyBytes() ([]byte, error) {
	key, err := parseOIDCEncryptionKey(c.EncryptionKey)
	if err != nil {
		return nil, RedactedError("auth.oidc.encryptionKey", err.Error())
	}
	return key, nil
}

func (c OIDCConfig) validate(production bool) error {
	if c.HTTPTimeout <= 0 {
		return RedactedError("auth.oidc.httpTimeout", "must be positive")
	}
	if c.TransactionTTL <= 0 {
		return RedactedError("auth.oidc.transactionTTL", "must be positive")
	}
	if c.GrantTTL <= 0 {
		return RedactedError("auth.oidc.grantTTL", "must be positive")
	}
	if strings.TrimSpace(c.CookieName) == "" {
		return RedactedError("auth.oidc.cookieName", "is required")
	}
	if strings.TrimSpace(c.CookiePath) == "" {
		return RedactedError("auth.oidc.cookiePath", "is required")
	}
	if !c.Enabled {
		return nil
	}
	if _, err := parseOIDCEncryptionKey(c.EncryptionKey); err != nil {
		return RedactedError("auth.oidc.encryptionKey", err.Error())
	}
	if looksLikePlaceholder(c.EncryptionKey) {
		return RedactedError("auth.oidc.encryptionKey", "rejects placeholder values")
	}
	if len(c.Providers) == 0 {
		return RedactedError("auth.oidc.providers", "must not be empty when OIDC is enabled")
	}
	seen := map[string]struct{}{}
	for _, provider := range c.Providers {
		name := strings.ToLower(strings.TrimSpace(provider.Name))
		if name == "" {
			return RedactedError("auth.oidc.providers.name", "is required")
		}
		if _, dup := seen[name]; dup {
			return RedactedError("auth.oidc.providers.name", "must be unique")
		}
		seen[name] = struct{}{}
		if err := provider.validate(production); err != nil {
			return err
		}
	}
	return nil
}

func (p OIDCProviderConfig) validate(production bool) error {
	if strings.TrimSpace(p.ClientID) == "" {
		return RedactedError("auth.oidc.providers.clientID", "is required")
	}
	if strings.TrimSpace(p.ClientSecret) == "" {
		return RedactedError("auth.oidc.providers.clientSecret", "is required")
	}
	if looksLikePlaceholder(p.ClientSecret) {
		return RedactedError("auth.oidc.providers.clientSecret", "rejects placeholder values")
	}
	if err := validateAbsoluteURL("auth.oidc.providers.issuer", p.Issuer, production, false); err != nil {
		return err
	}
	if err := validateAbsoluteURL("auth.oidc.providers.redirectURI", p.RedirectURI, production, true); err != nil {
		return err
	}
	if len(p.AllowedFinalDestinations) == 0 {
		return RedactedError("auth.oidc.providers.allowedFinalDestinations", "must not be empty")
	}
	for _, dest := range p.AllowedFinalDestinations {
		if err := validateAbsoluteURL("auth.oidc.providers.allowedFinalDestinations", dest, production, true); err != nil {
			return err
		}
	}
	return nil
}

func validateAbsoluteURL(field, raw string, production, rejectQuery bool) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return RedactedError(field, "is required")
	}
	if strings.Contains(value, "*") {
		return RedactedError(field, "must not contain wildcards")
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return RedactedError(field, "must be an absolute URL")
	}
	if u.User != nil {
		return RedactedError(field, "must not include credentials")
	}
	if rejectQuery && (u.RawQuery != "" || u.Fragment != "") {
		return RedactedError(field, "must not include query or fragment")
	}
	scheme := strings.ToLower(u.Scheme)
	if production && scheme != "https" {
		return RedactedError(field, "must use https in production")
	}
	if scheme != "https" && scheme != "http" {
		return RedactedError(field, "must use http or https")
	}
	return nil
}

func parseOIDCEncryptionKey(raw string) ([]byte, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, errOIDCKeyRequired
	}
	if looksLikePlaceholder(value) {
		return nil, errOIDCKeyPlaceholder
	}
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	return nil, errOIDCKeyInvalid
}

type oidcKeyError string

func (e oidcKeyError) Error() string { return string(e) }

const (
	errOIDCKeyRequired    oidcKeyError = "is required when OIDC is enabled"
	errOIDCKeyPlaceholder oidcKeyError = "rejects placeholder values"
	errOIDCKeyInvalid     oidcKeyError = "must be 32 bytes as hex or base64"
)
