package bootstrap

import (
	"strings"
	"testing"

	"github.com/WilsonSayago/authBase/v4/infra/config/properties"
	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
)

func TestNewTokenMaterialHMACHasNoJWKS(t *testing.T) {
	t.Parallel()
	material, err := newTokenMaterial(properties.Jwt{
		SecretKey:        "change-me-with-32-random-bytes-access-token-value",
		RefreshSecret:    "change-me-with-32-random-bytes-refresh-token-value",
		ExpirationTime:   1,
		RefreshTokenTime: 24,
		Issuer:           "issuer",
		Audience:         "audience",
	}, appconfig.TokenKeysConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(material.JWKS) != 0 {
		t.Fatalf("hmac must not publish jwks: %s", material.JWKS)
	}
	if material.Crypto.AccessSigner == nil || material.Crypto.AccessVerifier == nil {
		t.Fatal("hmac crypto is required")
	}
}

func TestTokenKeysRejectsEd25519PathsOnHMAC(t *testing.T) {
	t.Parallel()
	cfg := appconfig.Defaults()
	cfg.Database.Password = "secret-password-value"
	cfg.Database.SSLMode = "disable"
	cfg.Auth.TokenKeys.AccessKeyPath = "/tmp/access.pem"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "auth.tokenKeys.algorithm") {
		t.Fatalf("err=%v", err)
	}
}
