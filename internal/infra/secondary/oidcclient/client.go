package oidcclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/appconfig"
	"golang.org/x/oauth2"
)

// MaxIDTokenIATLeeway is the accepted clock skew for ID-token iat claims.
const MaxIDTokenIATLeeway = 2 * time.Minute

// Client is a provider-neutral Authorization Code + PKCE S256 adapter.
type Client struct {
	name        string
	issuer      string
	redirectURI string
	oauth       oauth2.Config
	timeout     time.Duration
	httpClient  *http.Client
	mu          sync.Mutex
	provider    *oidc.Provider
	verifier    *oidc.IDTokenVerifier
}

func New(cfg appconfig.OIDCProviderConfig, timeout time.Duration, httpClient *http.Client) (*Client, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Name))
	if name == "" || cfg.Issuer == "" || cfg.ClientID == "" || cfg.RedirectURI == "" {
		return nil, fmt.Errorf("oidc provider configuration is incomplete")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	} else if httpClient.Timeout == 0 {
		cloned := *httpClient
		cloned.Timeout = timeout
		httpClient = &cloned
	}
	return &Client{
		name:        name,
		issuer:      strings.TrimRight(cfg.Issuer, "/"),
		redirectURI: cfg.RedirectURI,
		timeout:     timeout,
		httpClient:  httpClient,
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURI,
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
	}, nil
}

func (c *Client) Name() string   { return c.name }
func (c *Client) Issuer() string { return c.issuer }

func (c *Client) AuthorizationURL(state, nonce, challenge string) (string, error) {
	if state == "" || nonce == "" || challenge == "" {
		return "", fmt.Errorf("oidc authorization parameters are required")
	}
	if err := c.ensureProvider(context.Background()); err != nil {
		return "", err
	}
	return c.oauth.AuthCodeURL(state,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	), nil
}

func (c *Client) Exchange(ctx context.Context, code, verifier, nonce string) (domain.ExternalPrincipal, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(verifier) == "" || strings.TrimSpace(nonce) == "" {
		return domain.ExternalPrincipal{}, domain.ErrOIDCInvalidState
	}
	if err := c.ensureProvider(ctx); err != nil {
		return domain.ExternalPrincipal{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	ctx = oidc.ClientContext(ctx, c.httpClient)
	token, err := c.oauth.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return domain.ExternalPrincipal{}, domain.ErrOIDCInvalidState
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return domain.ExternalPrincipal{}, domain.ErrOIDCInvalidState
	}
	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return domain.ExternalPrincipal{}, domain.ErrOIDCInvalidState
	}
	if err := validateIDTokenClaims(idToken, c.issuer, nonce); err != nil {
		return domain.ExternalPrincipal{}, err
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return domain.ExternalPrincipal{}, domain.ErrOIDCInvalidState
	}
	return domain.ExternalPrincipal{
		Provider:      c.name,
		Issuer:        idToken.Issuer,
		Subject:       idToken.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
	}, nil
}

func validateIDTokenClaims(idToken *oidc.IDToken, expectedIssuer, nonce string) error {
	if idToken == nil {
		return domain.ErrOIDCInvalidState
	}
	if strings.TrimSpace(idToken.Subject) == "" {
		return domain.ErrOIDCInvalidState
	}
	if idToken.IssuedAt.IsZero() {
		return domain.ErrOIDCInvalidState
	}
	if idToken.IssuedAt.After(time.Now().Add(MaxIDTokenIATLeeway)) {
		return domain.ErrOIDCInvalidState
	}
	if idToken.Nonce != nonce {
		return domain.ErrOIDCInvalidState
	}
	if !strings.EqualFold(strings.TrimRight(idToken.Issuer, "/"), expectedIssuer) {
		return domain.ErrOIDCInvalidState
	}
	return nil
}

func (c *Client) ensureProvider(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.provider != nil && c.verifier != nil && c.oauth.Endpoint.AuthURL != "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	ctx = oidc.ClientContext(ctx, c.httpClient)
	provider, err := oidc.NewProvider(ctx, c.issuer)
	if err != nil {
		return domain.ErrOIDCUnavailable
	}
	c.provider = provider
	c.verifier = provider.Verifier(&oidc.Config{ClientID: c.oauth.ClientID})
	c.oauth.Endpoint = provider.Endpoint()
	return nil
}

var _ ports.OIDCProvider = (*Client)(nil)
