package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBasePorts "github.com/WilsonSayago/authBase/v4/core/port"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
)

// OIDCService orchestrates provider-neutral login and explicit account linking.
type OIDCService struct {
	enabled        bool
	transactionTTL time.Duration
	grantTTL       time.Duration
	cookieTTL      time.Duration
	providers      map[string]ports.OIDCProvider
	destinations   map[string][]string
	txns           ports.OIDCTransactionStore
	identities     ports.ExternalIdentityStore
	users          oidcUserReader
	secrets        ports.OIDCSecrets
	sessions       authBaseCore.SessionIssuer
	validation     authBasePorts.ValidationPort
	audit          ports.OIDCAuditor
	now            func() time.Time
}

type OIDCServiceConfig struct {
	Enabled        bool
	TransactionTTL time.Duration
	GrantTTL       time.Duration
}

type OIDCStart struct {
	AuthorizationURL string
	State            string
	CookieTTL        time.Duration
}

type OIDCCallback struct {
	RedirectURL string
}

type oidcUserReader interface {
	FindByID(ctx context.Context, id string) (domain.User, error)
	FindCredentialsByUsername(ctx context.Context, username string) (authBasePorts.CredentialRecord, error)
}

func NewOIDCService(
	cfg OIDCServiceConfig,
	providers []ports.OIDCProvider,
	destinations map[string][]string,
	txns ports.OIDCTransactionStore,
	identities ports.ExternalIdentityStore,
	users oidcUserReader,
	secrets ports.OIDCSecrets,
	sessions authBaseCore.SessionIssuer,
	validation authBasePorts.ValidationPort,
	audit ports.OIDCAuditor,
) (*OIDCService, error) {
	if cfg.Enabled {
		if cfg.TransactionTTL <= 0 || cfg.GrantTTL <= 0 {
			return nil, fmt.Errorf("oidc ttls must be positive")
		}
		if txns == nil || identities == nil || users == nil || secrets == nil || sessions == nil || validation == nil {
			return nil, fmt.Errorf("oidc dependencies are required")
		}
		if len(providers) == 0 {
			return nil, fmt.Errorf("oidc providers are required")
		}
	}
	if audit == nil {
		audit = ports.NoopOIDCAuditor{}
	}
	byName := make(map[string]ports.OIDCProvider, len(providers))
	for _, provider := range providers {
		if provider == nil {
			return nil, fmt.Errorf("oidc provider is nil")
		}
		byName[strings.ToLower(provider.Name())] = provider
	}
	if destinations == nil {
		destinations = map[string][]string{}
	}
	return &OIDCService{
		enabled:        cfg.Enabled,
		transactionTTL: cfg.TransactionTTL,
		grantTTL:       cfg.GrantTTL,
		cookieTTL:      cfg.TransactionTTL,
		providers:      byName,
		destinations:   destinations,
		txns:           txns,
		identities:     identities,
		users:          users,
		secrets:        secrets,
		sessions:       sessions,
		validation:     validation,
		audit:          audit,
		now:            time.Now,
	}, nil
}

func (s *OIDCService) StartLogin(ctx context.Context, providerName, returnTo string) (OIDCStart, error) {
	return s.start(ctx, providerName, domain.OIDCIntentLogin, "", "", returnTo)
}

func (s *OIDCService) StartLink(ctx context.Context, providerName, actorUserID, password, returnTo string) (OIDCStart, error) {
	if strings.TrimSpace(actorUserID) == "" {
		return OIDCStart{}, domain.ErrOIDCReauthRequired
	}
	if err := s.verifyPassword(ctx, actorUserID, password); err != nil {
		s.audit.OIDCEvent(domain.OIDCOutcomeLink, providerName, actorUserID, domain.OIDCOutcomeFailure)
		return OIDCStart{}, err
	}
	return s.start(ctx, providerName, domain.OIDCIntentLink, actorUserID, password, returnTo)
}

func (s *OIDCService) start(
	ctx context.Context,
	providerName string,
	intent domain.OIDCIntent,
	actorUserID, _ /*password*/, returnTo string,
) (OIDCStart, error) {
	if err := ctx.Err(); err != nil {
		return OIDCStart{}, err
	}
	provider, dests, err := s.provider(providerName)
	if err != nil {
		return OIDCStart{}, err
	}
	normalized, err := domain.NormalizeReturnTo(returnTo, dests)
	if err != nil {
		s.audit.OIDCEvent(domain.OIDCOutcomeStart, provider.Name(), actorUserID, domain.OIDCOutcomeFailure)
		return OIDCStart{}, err
	}
	state, stateHash, err := randomSecret()
	if err != nil {
		return OIDCStart{}, err
	}
	nonce, _, err := randomSecret()
	if err != nil {
		return OIDCStart{}, err
	}
	verifier, challenge, err := pkcePair()
	if err != nil {
		return OIDCStart{}, err
	}
	sealed, err := s.secrets.Seal([]byte(verifier))
	if err != nil {
		return OIDCStart{}, fmt.Errorf("seal pkce verifier: %w", err)
	}
	authURL, err := provider.AuthorizationURL(state, nonce, challenge)
	if err != nil {
		s.audit.OIDCEvent(domain.OIDCOutcomeStart, provider.Name(), actorUserID, domain.OIDCOutcomeFailure)
		return OIDCStart{}, err
	}
	now := s.now().UTC()
	if err := s.txns.Create(ctx, domain.OIDCTransaction{
		Provider:       provider.Name(),
		StateHash:      stateHash,
		Nonce:          []byte(nonce),
		VerifierCipher: sealed,
		Intent:         intent,
		ActorUserID:    actorUserID,
		ReturnTo:       normalized,
		ExpiresAt:      now.Add(s.transactionTTL),
	}); err != nil {
		return OIDCStart{}, err
	}
	s.audit.OIDCEvent(domain.OIDCOutcomeStart, provider.Name(), actorUserID, domain.OIDCOutcomeStart)
	return OIDCStart{
		AuthorizationURL: authURL,
		State:            state,
		CookieTTL:        s.cookieTTL,
	}, nil
}

func (s *OIDCService) FinishCallback(ctx context.Context, providerName, cookieState, queryState, code string) (OIDCCallback, error) {
	if err := ctx.Err(); err != nil {
		return OIDCCallback{}, err
	}
	provider, _, err := s.provider(providerName)
	if err != nil {
		return OIDCCallback{}, err
	}
	if cookieState == "" || queryState == "" || cookieState != queryState {
		s.audit.OIDCEvent(domain.OIDCOutcomeFailure, provider.Name(), "", domain.OIDCOutcomeFailure)
		return OIDCCallback{}, domain.ErrOIDCInvalidState
	}
	if strings.TrimSpace(code) == "" {
		s.audit.OIDCEvent(domain.OIDCOutcomeCancelled, provider.Name(), "", domain.OIDCOutcomeFailure)
		return OIDCCallback{}, domain.ErrOIDCInvalidState
	}
	now := s.now().UTC()
	tx, err := s.txns.ConsumeByStateHash(ctx, hashSecret(cookieState), now)
	if err != nil {
		s.audit.OIDCEvent(domain.OIDCOutcomeFailure, provider.Name(), "", domain.OIDCOutcomeFailure)
		return OIDCCallback{}, err
	}
	if !strings.EqualFold(tx.Provider, provider.Name()) {
		return OIDCCallback{}, domain.ErrOIDCInvalidState
	}
	verifier, err := s.secrets.Open(tx.VerifierCipher)
	if err != nil {
		return OIDCCallback{}, fmt.Errorf("open pkce verifier: %w", err)
	}
	principal, err := provider.Exchange(ctx, code, string(verifier), string(tx.Nonce))
	if err != nil {
		s.audit.OIDCEvent(domain.OIDCOutcomeFailure, provider.Name(), tx.ActorUserID, domain.OIDCOutcomeFailure)
		return OIDCCallback{}, err
	}
	if principal.Email != "" && !principal.EmailVerified {
		s.audit.OIDCEvent(domain.OIDCOutcomeFailure, provider.Name(), tx.ActorUserID, domain.OIDCOutcomeFailure)
		return OIDCCallback{}, domain.ErrOIDCEmailUnverified
	}
	switch tx.Intent {
	case domain.OIDCIntentLink:
		return s.finishLink(ctx, tx, principal, now)
	default:
		return s.finishLogin(ctx, tx, principal, now)
	}
}

func (s *OIDCService) finishLogin(ctx context.Context, tx domain.OIDCTransaction, principal domain.ExternalPrincipal, now time.Time) (OIDCCallback, error) {
	identity, err := s.identities.FindByIssuerSubject(ctx, principal.Issuer, principal.Subject)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			s.audit.OIDCEvent(domain.OIDCOutcomeFailure, tx.Provider, "", domain.OIDCOutcomeFailure)
			return OIDCCallback{}, domain.ErrOIDCIdentityNotLinked
		}
		return OIDCCallback{}, err
	}
	user, err := s.requireActiveUser(ctx, identity.UserID)
	if err != nil {
		s.audit.OIDCEvent(domain.OIDCOutcomeFailure, tx.Provider, identity.UserID, domain.OIDCOutcomeFailure)
		return OIDCCallback{}, err
	}
	if err := s.identities.MarkLogin(ctx, identity.ID, now); err != nil {
		return OIDCCallback{}, err
	}
	redirect, err := s.attachGrant(ctx, tx, user.GetId(), now)
	if err != nil {
		return OIDCCallback{}, err
	}
	s.audit.OIDCEvent(domain.OIDCOutcomeSuccess, tx.Provider, user.GetId(), domain.OIDCOutcomeSuccess)
	return OIDCCallback{RedirectURL: redirect}, nil
}

func (s *OIDCService) finishLink(ctx context.Context, tx domain.OIDCTransaction, principal domain.ExternalPrincipal, now time.Time) (OIDCCallback, error) {
	if tx.ActorUserID == "" {
		return OIDCCallback{}, domain.ErrOIDCReauthRequired
	}
	if _, err := s.requireActiveUser(ctx, tx.ActorUserID); err != nil {
		s.audit.OIDCEvent(domain.OIDCOutcomeLink, tx.Provider, tx.ActorUserID, domain.OIDCOutcomeFailure)
		return OIDCCallback{}, err
	}
	existing, err := s.identities.FindByIssuerSubject(ctx, principal.Issuer, principal.Subject)
	if err == nil {
		if existing.UserID != tx.ActorUserID {
			s.audit.OIDCEvent(domain.OIDCOutcomeConflict, tx.Provider, tx.ActorUserID, domain.OIDCOutcomeConflict)
			return OIDCCallback{}, domain.ErrOIDCIdentityConflict
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return OIDCCallback{}, err
	} else if err := s.identities.Link(ctx, domain.ExternalIdentity{
		Provider:    tx.Provider,
		Issuer:      principal.Issuer,
		Subject:     principal.Subject,
		UserID:      tx.ActorUserID,
		EmailAtLink: principal.Email,
	}); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			s.audit.OIDCEvent(domain.OIDCOutcomeConflict, tx.Provider, tx.ActorUserID, domain.OIDCOutcomeConflict)
			return OIDCCallback{}, domain.ErrOIDCIdentityConflict
		}
		return OIDCCallback{}, err
	}
	redirect, err := appendOIDCStatus(tx.ReturnTo, "linked")
	if err != nil {
		return OIDCCallback{}, err
	}
	s.audit.OIDCEvent(domain.OIDCOutcomeLink, tx.Provider, tx.ActorUserID, domain.OIDCOutcomeSuccess)
	_ = now
	return OIDCCallback{RedirectURL: redirect}, nil
}

func (s *OIDCService) ExchangeGrant(ctx context.Context, grant string) (accessToken, refreshToken string, err error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	if !s.enabled {
		return "", "", domain.ErrOIDCDisabled
	}
	if strings.TrimSpace(grant) == "" {
		return "", "", domain.ErrOIDCInvalidGrant
	}
	tx, err := s.txns.ConsumeGrant(ctx, hashSecret(grant), s.now().UTC())
	if err != nil {
		s.audit.OIDCEvent(domain.OIDCOutcomeFailure, "", "", domain.OIDCOutcomeFailure)
		return "", "", err
	}
	if _, err := s.requireActiveUser(ctx, tx.ResolvedUserID); err != nil {
		s.audit.OIDCEvent(domain.OIDCOutcomeFailure, tx.Provider, tx.ResolvedUserID, domain.OIDCOutcomeFailure)
		return "", "", err
	}
	access, refresh, err := s.sessions.EstablishSession(ctx, tx.ResolvedUserID)
	if err != nil {
		if errors.Is(err, authBaseCore.ErrInactiveIdentity) || errors.Is(err, authBaseCore.ErrNotFound) {
			return "", "", domain.ErrOIDCInactiveUser
		}
		return "", "", err
	}
	s.audit.OIDCEvent(domain.OIDCOutcomeSuccess, tx.Provider, tx.ResolvedUserID, domain.OIDCOutcomeSuccess)
	return access, refresh, nil
}

func (s *OIDCService) Unlink(ctx context.Context, providerName, actorUserID, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, _, err := s.provider(providerName); err != nil {
		return err
	}
	if err := s.verifyPassword(ctx, actorUserID, password); err != nil {
		s.audit.OIDCEvent(domain.OIDCOutcomeUnlink, providerName, actorUserID, domain.OIDCOutcomeFailure)
		return err
	}
	if err := s.identities.Unlink(ctx, actorUserID, providerName); err != nil {
		s.audit.OIDCEvent(domain.OIDCOutcomeUnlink, providerName, actorUserID, domain.OIDCOutcomeFailure)
		return err
	}
	s.audit.OIDCEvent(domain.OIDCOutcomeUnlink, providerName, actorUserID, domain.OIDCOutcomeSuccess)
	return nil
}

func (s *OIDCService) attachGrant(ctx context.Context, tx domain.OIDCTransaction, userID string, now time.Time) (string, error) {
	grant, grantHash, err := randomSecret()
	if err != nil {
		return "", err
	}
	if err := s.txns.AttachGrant(ctx, tx.ID, grantHash, userID, now.Add(s.grantTTL)); err != nil {
		return "", err
	}
	return appendGrant(tx.ReturnTo, grant)
}

func (s *OIDCService) provider(name string) (ports.OIDCProvider, []string, error) {
	if !s.enabled {
		return nil, nil, domain.ErrOIDCDisabled
	}
	key := strings.ToLower(strings.TrimSpace(name))
	provider, ok := s.providers[key]
	if !ok {
		return nil, nil, domain.ErrOIDCUnknownProvider
	}
	return provider, s.destinations[key], nil
}

func (s *OIDCService) requireActiveUser(ctx context.Context, userID string) (domain.User, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, authBaseCore.ErrNotFound) {
			return domain.User{}, domain.ErrOIDCIdentityNotLinked
		}
		return domain.User{}, err
	}
	if !user.GetActive() {
		return domain.User{}, domain.ErrOIDCInactiveUser
	}
	return user, nil
}

func (s *OIDCService) verifyPassword(ctx context.Context, userID, password string) error {
	if strings.TrimSpace(password) == "" {
		return domain.ErrOIDCReauthRequired
	}
	user, err := s.requireActiveUser(ctx, userID)
	if err != nil {
		return err
	}
	cred, err := s.users.FindCredentialsByUsername(ctx, user.GetEmail())
	if err != nil {
		return domain.ErrOIDCReauthRequired
	}
	if !cred.Active || !s.validation.CheckPassword(cred.PasswordHash, password) {
		return domain.ErrOIDCReauthRequired
	}
	return nil
}

func randomSecret() (raw string, hash []byte, err error) {
	var buf [32]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		return "", nil, fmt.Errorf("generate secret: %w", err)
	}
	raw = hex.EncodeToString(buf[:])
	return raw, hashSecret(raw), nil
}

func hashSecret(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func pkcePair() (verifier, challenge string, err error) {
	var buf [32]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		return "", "", fmt.Errorf("generate pkce: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf[:])
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func appendGrant(returnTo, grant string) (string, error) {
	return appendQuery(returnTo, "grant", grant)
}

func appendOIDCStatus(returnTo, status string) (string, error) {
	return appendQuery(returnTo, "oidc", status)
}

func appendQuery(raw, key, value string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", domain.ErrOIDCRedirectNotAllowed
	}
	q := parsed.Query()
	q.Set(key, value)
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}
