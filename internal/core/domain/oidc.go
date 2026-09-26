package domain

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

const (
	OIDCIntentLogin OIDCIntent = "login"
	OIDCIntentLink  OIDCIntent = "link"

	OIDCOutcomeStart     = "start"
	OIDCOutcomeSuccess   = "success"
	OIDCOutcomeFailure   = "failure"
	OIDCOutcomeLink      = "link"
	OIDCOutcomeConflict  = "conflict"
	OIDCOutcomeUnlink    = "unlink"
	OIDCOutcomeCancelled = "cancelled"
)

type OIDCIntent string

// ExternalPrincipal is the verified (issuer, subject) from an ID token.
type ExternalPrincipal struct {
	Provider      string
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
}

// ExternalIdentity is a persisted link between a local user and a principal.
type ExternalIdentity struct {
	ID          string
	Provider    string
	Issuer      string
	Subject     string
	UserID      string
	EmailAtLink string
	LastLoginAt *time.Time
	CreatedAt   time.Time
}

// OIDCTransaction is a single-use Authorization Code start record.
type OIDCTransaction struct {
	ID              string
	Provider        string
	StateHash       []byte
	Nonce           []byte
	VerifierCipher  []byte
	Intent          OIDCIntent
	ActorUserID     string
	ReturnTo        string
	ExpiresAt       time.Time
	ConsumedAt      *time.Time
	GrantHash       []byte
	GrantExpiresAt  *time.Time
	GrantConsumedAt *time.Time
	ResolvedUserID  string
}

var (
	ErrOIDCDisabled           = errors.New("oidc is disabled")
	ErrOIDCUnavailable        = errors.New("oidc provider unavailable")
	ErrOIDCUnknownProvider    = errors.New("unknown oidc provider")
	ErrOIDCInvalidState       = errors.New("invalid oidc state")
	ErrOIDCExpired            = errors.New("oidc transaction expired")
	ErrOIDCReplay             = errors.New("oidc artifact already consumed")
	ErrOIDCRedirectNotAllowed = errors.New("oidc redirect is not allowlisted")
	ErrOIDCIdentityNotLinked  = errors.New("oidc identity is not linked")
	ErrOIDCIdentityConflict   = errors.New("oidc identity already linked")
	ErrOIDCEmailUnverified    = errors.New("oidc email is not verified")
	ErrOIDCReauthRequired     = errors.New("oidc link requires recent password confirmation")
	ErrOIDCInactiveUser       = errors.New("oidc user is inactive")
	ErrOIDCInvalidGrant       = errors.New("invalid oidc grant")
)

// NormalizeReturnTo validates an exact allowlisted final destination.
func NormalizeReturnTo(raw string, allowed []string) (string, error) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		if len(allowed) == 1 {
			return allowed[0], nil
		}
		return "", ErrOIDCRedirectNotAllowed
	}
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return "", ErrOIDCRedirectNotAllowed
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrOIDCRedirectNotAllowed
	}
	normalized := parsed.String()
	for _, dest := range allowed {
		if dest == normalized {
			return normalized, nil
		}
	}
	return "", ErrOIDCRedirectNotAllowed
}

// PrincipalKey is the stable federated identity.
func (p ExternalPrincipal) PrincipalKey() (issuer, subject string) {
	return strings.TrimSpace(p.Issuer), strings.TrimSpace(p.Subject)
}
