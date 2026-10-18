package oidcclient_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/oidcclient"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

func TestClientExchangeVerifiesIDToken(t *testing.T) {
	provider, issuer := startFakeOIDC(t, fakeOIDCConfig{
		ClientID: "client-1", Code: "good-code", Verifier: "good-verifier",
		Nonce: "good-nonce", Subject: "sub-9", Verified: true,
	})
	defer provider.Close()

	client, err := oidcclient.New(appconfig.OIDCProviderConfig{
		Name:         "google",
		Issuer:       issuer,
		ClientID:     "client-1",
		ClientSecret: "secret",
		RedirectURI:  "https://api.example.com/v1/auth/oidc/google/callback",
	}, 2*time.Second, provider.Client())
	if err != nil {
		t.Fatal(err)
	}
	principal, err := client.Exchange(context.Background(), "good-code", "good-verifier", "good-nonce")
	if err != nil {
		t.Fatal(err)
	}
	if principal.Subject != "sub-9" || principal.Issuer != issuer || !principal.EmailVerified {
		t.Fatalf("principal=%+v", principal)
	}

	if _, err := client.Exchange(context.Background(), "good-code", "good-verifier", "bad-nonce"); err != domain.ErrOIDCInvalidState {
		t.Fatalf("nonce err=%v", err)
	}
}

func TestClientRejectsUnknownIssuer(t *testing.T) {
	provider, issuer := startFakeOIDC(t, fakeOIDCConfig{
		ClientID: "client-1", Code: "good-code", Verifier: "good-verifier",
		Nonce: "good-nonce", Subject: "sub-9", Verified: true,
	})
	defer provider.Close()

	client, err := oidcclient.New(appconfig.OIDCProviderConfig{
		Name:         "google",
		Issuer:       issuer,
		ClientID:     "other-client",
		ClientSecret: "secret",
		RedirectURI:  "https://api.example.com/callback",
	}, 2*time.Second, provider.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Exchange(context.Background(), "good-code", "good-verifier", "good-nonce"); err != domain.ErrOIDCInvalidState {
		t.Fatalf("aud err=%v", err)
	}
}

func TestClientRejectsInvalidClaims(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cfg  fakeOIDCConfig
	}{
		{name: "empty subject", cfg: fakeOIDCConfig{Subject: ""}},
		{name: "missing iat", cfg: fakeOIDCConfig{OmitIAT: true}},
		{name: "future iat", cfg: fakeOIDCConfig{IssuedAt: time.Now().Add(time.Hour)}},
		{name: "expired", cfg: fakeOIDCConfig{Expiry: time.Now().Add(-time.Minute)}},
		{name: "wrong signature", cfg: fakeOIDCConfig{WrongSignature: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := tc.cfg
			cfg.ClientID = "client-1"
			cfg.Code = "good-code"
			cfg.Verifier = "good-verifier"
			cfg.Nonce = "good-nonce"
			if tc.name != "empty subject" {
				cfg.Subject = "sub-9"
			}
			cfg.Verified = true
			provider, issuer := startFakeOIDC(t, cfg)
			defer provider.Close()
			client, err := oidcclient.New(appconfig.OIDCProviderConfig{
				Name: "google", Issuer: issuer, ClientID: "client-1", ClientSecret: "secret",
				RedirectURI: "https://api.example.com/callback",
			}, 2*time.Second, provider.Client())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Exchange(context.Background(), "good-code", "good-verifier", "good-nonce"); err != domain.ErrOIDCInvalidState {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestClientExchangeHonorsTimeoutAndCancel(t *testing.T) {
	t.Parallel()
	provider, issuer := startFakeOIDC(t, fakeOIDCConfig{
		ClientID: "client-1", Code: "good-code", Verifier: "good-verifier",
		Nonce: "good-nonce", Subject: "sub-9", Verified: true, HangToken: true,
	})
	defer provider.Close()
	client, err := oidcclient.New(appconfig.OIDCProviderConfig{
		Name: "google", Issuer: issuer, ClientID: "client-1", ClientSecret: "secret",
		RedirectURI: "https://api.example.com/callback",
	}, 50*time.Millisecond, provider.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.AuthorizationURL("state", "good-nonce", "challenge"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Exchange(context.Background(), "good-code", "good-verifier", "good-nonce"); err == nil {
		t.Fatal("expected timeout")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Exchange(ctx, "good-code", "good-verifier", "good-nonce"); err == nil {
		t.Fatal("expected canceled exchange")
	}
}

type fakeOIDCConfig struct {
	ClientID, Code, Verifier, Nonce, Subject     string
	Verified, OmitIAT, WrongSignature, HangToken bool
	IssuedAt, Expiry                             time.Time
}

func startFakeOIDC(t *testing.T, cfg fakeOIDCConfig) (*httptest.Server, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	altKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	var server *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                server.URL,
			"authorization_endpoint":                server.URL + "/auth",
			"token_endpoint":                        server.URL + "/token",
			"jwks_uri":                              server.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key: &key.PublicKey, KeyID: "kid-1", Algorithm: string(jose.RS256), Use: "sig",
		}}}
		_ = json.NewEncoder(w).Encode(jwks)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if cfg.HangToken {
			time.Sleep(time.Second)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "form", http.StatusBadRequest)
			return
		}
		if r.Form.Get("code") != cfg.Code || r.Form.Get("code_verifier") != cfg.Verifier {
			http.Error(w, "code", http.StatusBadRequest)
			return
		}
		signKey := key
		if cfg.WrongSignature {
			signKey = altKey
		}
		signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: signKey}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "kid-1"))
		if err != nil {
			http.Error(w, "sign", http.StatusInternalServerError)
			return
		}
		now := time.Now()
		exp := now.Add(time.Minute)
		if !cfg.Expiry.IsZero() {
			exp = cfg.Expiry
		}
		iat := now
		if !cfg.IssuedAt.IsZero() {
			iat = cfg.IssuedAt
		}
		claims := map[string]any{
			"iss":            server.URL,
			"sub":            cfg.Subject,
			"aud":            cfg.ClientID,
			"exp":            exp.Unix(),
			"nonce":          cfg.Nonce,
			"email":          "ada@example.com",
			"email_verified": cfg.Verified,
		}
		if !cfg.OmitIAT {
			claims["iat"] = iat.Unix()
		}
		raw, err := jwt.Signed(signer).Claims(claims).Serialize()
		if err != nil {
			http.Error(w, "jwt", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "not-stored",
			"token_type":   "Bearer",
			"expires_in":   60,
			"id_token":     raw,
		})
	})
	server = httptest.NewServer(mux)
	return server, server.URL
}
