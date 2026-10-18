package ports

import (
	"context"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// OIDCTransactionStore persists single-use Authorization Code transactions and grants.
type OIDCTransactionStore interface {
	Create(ctx context.Context, tx domain.OIDCTransaction) error
	ConsumeByStateHash(ctx context.Context, stateHash []byte, at time.Time) (domain.OIDCTransaction, error)
	AttachGrant(ctx context.Context, id string, grantHash []byte, userID string, expiresAt time.Time) error
	ConsumeGrant(ctx context.Context, grantHash []byte, at time.Time) (domain.OIDCTransaction, error)
	PurgeExpired(ctx context.Context, before time.Time) (int64, error)
}

// ExternalIdentityStore resolves and links (issuer, subject) to a local user.
type ExternalIdentityStore interface {
	FindByIssuerSubject(ctx context.Context, issuer, subject string) (domain.ExternalIdentity, error)
	Link(ctx context.Context, identity domain.ExternalIdentity) error
	MarkLogin(ctx context.Context, id string, at time.Time) error
	Unlink(ctx context.Context, userID, provider string) error
}

// OIDCProvider starts Authorization Code + PKCE and exchanges a code for a principal.
type OIDCProvider interface {
	Name() string
	Issuer() string
	AuthorizationURL(state, nonce, challenge string) (string, error)
	Exchange(ctx context.Context, code, verifier, nonce string) (domain.ExternalPrincipal, error)
}

// OIDCSecrets seals recoverable PKCE verifiers at rest.
type OIDCSecrets interface {
	Seal(plaintext []byte) ([]byte, error)
	Open(ciphertext []byte) ([]byte, error)
}

// OIDCAuditor records typed federated-login events without tokens or raw PII.
type OIDCAuditor interface {
	OIDCEvent(action, provider, actorID, outcome string)
}

// NoopOIDCAuditor discards OIDC audit events.
type NoopOIDCAuditor struct{}

func (NoopOIDCAuditor) OIDCEvent(string, string, string, string) {}
