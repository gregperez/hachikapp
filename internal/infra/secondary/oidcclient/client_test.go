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
	provider, issuer := newFakeOIDC(t, "client-1", "good-code", "good-verifier", "good-nonce", "sub-9", true)
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
	provider, issuer := newFakeOIDC(t, "client-1", "good-code", "good-verifier", "good-nonce", "sub-9", true)
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

type fakeOIDC struct {
	key      *rsa.PrivateKey
	issuer   string
	clientID string
	code     string
	verifier string
	nonce    string
	subject  string
	verified bool
}

func newFakeOIDC(t *testing.T, clientID, code, verifier, nonce, subject string, verified bool) (*httptest.Server, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	state := &fakeOIDC{
		key: key, clientID: clientID, code: code, verifier: verifier, nonce: nonce, subject: subject, verified: verified,
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
		if err := r.ParseForm(); err != nil {
			http.Error(w, "form", http.StatusBadRequest)
			return
		}
		if r.Form.Get("code") != state.code || r.Form.Get("code_verifier") != state.verifier {
			http.Error(w, "code", http.StatusBadRequest)
			return
		}
		signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "kid-1"))
		if err != nil {
			http.Error(w, "sign", http.StatusInternalServerError)
			return
		}
		now := time.Now()
		raw, err := jwt.Signed(signer).Claims(map[string]any{
			"iss":            server.URL,
			"sub":            state.subject,
			"aud":            state.clientID,
			"exp":            now.Add(time.Minute).Unix(),
			"iat":            now.Unix(),
			"nonce":          state.nonce,
			"email":          "ada@example.com",
			"email_verified": state.verified,
		}).Serialize()
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
	state.issuer = server.URL
	return server, server.URL
}
