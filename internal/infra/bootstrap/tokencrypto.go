package bootstrap

import (
	"fmt"

	authBaseServices "github.com/WilsonSayago/authBase/v4/core/services"
	"github.com/WilsonSayago/authBase/v4/infra/config/properties"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/appconfig"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/jwtkeys"
)

type tokenMaterial struct {
	Crypto authBaseServices.TokenCrypto
	JWKS   []byte
}

func newTokenMaterial(jwt properties.Jwt, keys appconfig.TokenKeysConfig) (tokenMaterial, error) {
	switch keys.AlgorithmNorm() {
	case appconfig.TokenAlgHS256:
		crypto, err := hmacCrypto(jwt, keys)
		if err != nil {
			return tokenMaterial{}, err
		}
		return tokenMaterial{Crypto: crypto}, nil
	case appconfig.TokenAlgEd25519:
		return ed25519Material(jwt, keys)
	default:
		return tokenMaterial{}, fmt.Errorf("unsupported jwt algorithm")
	}
}

func hmacCrypto(jwt properties.Jwt, keys appconfig.TokenKeysConfig) (authBaseServices.TokenCrypto, error) {
	var previousAccess, previousRefresh []authBaseServices.HMACKey
	if keys.PreviousAccessSecret != "" {
		previousAccess = []authBaseServices.HMACKey{{
			ID:     keys.PreviousAccessKID,
			Secret: []byte(keys.PreviousAccessSecret),
		}}
	}
	if keys.PreviousRefreshSecret != "" {
		previousRefresh = []authBaseServices.HMACKey{{
			ID:     keys.PreviousRefreshKID,
			Secret: []byte(keys.PreviousRefreshSecret),
		}}
	}
	return authBaseServices.NewHMACCryptoWithKeys(
		authBaseServices.HMACKey{ID: keys.AccessKID, Secret: []byte(jwt.SecretKey)},
		authBaseServices.HMACKey{ID: keys.RefreshKID, Secret: []byte(jwt.RefreshSecret)},
		previousAccess,
		previousRefresh,
	)
}

func ed25519Material(jwt properties.Jwt, keys appconfig.TokenKeysConfig) (tokenMaterial, error) {
	accessPriv, err := jwtkeys.LoadPrivate(keys.AccessKeyPath)
	if err != nil {
		return tokenMaterial{}, fmt.Errorf("auth.tokenKeys.accessKeyPath: %w", err)
	}
	refreshPriv, err := jwtkeys.LoadPrivate(keys.RefreshKeyPath)
	if err != nil {
		return tokenMaterial{}, fmt.Errorf("auth.tokenKeys.refreshKeyPath: %w", err)
	}
	access := authBaseServices.Ed25519Key{ID: keys.AccessKID, PrivateKey: accessPriv}
	refresh := authBaseServices.Ed25519Key{ID: keys.RefreshKID, PrivateKey: refreshPriv}

	var previousAccess, previousRefresh []authBaseServices.Ed25519Key
	if keys.PreviousAccessPubPath != "" {
		pub, err := jwtkeys.LoadPublic(keys.PreviousAccessPubPath)
		if err != nil {
			return tokenMaterial{}, fmt.Errorf("auth.tokenKeys.previousAccessPubPath: %w", err)
		}
		previousAccess = []authBaseServices.Ed25519Key{{ID: keys.PreviousAccessKID, PublicKey: pub}}
	}
	if keys.PreviousRefreshPubPath != "" {
		pub, err := jwtkeys.LoadPublic(keys.PreviousRefreshPubPath)
		if err != nil {
			return tokenMaterial{}, fmt.Errorf("auth.tokenKeys.previousRefreshPubPath: %w", err)
		}
		previousRefresh = []authBaseServices.Ed25519Key{{ID: keys.PreviousRefreshKID, PublicKey: pub}}
	}

	edCrypto, err := authBaseServices.NewEd25519Crypto(access, refresh, previousAccess, previousRefresh)
	if err != nil {
		return tokenMaterial{}, err
	}
	jwks, err := authBaseServices.AccessPublicJWKS(access, previousAccess...)
	if err != nil {
		return tokenMaterial{}, err
	}

	crypto := edCrypto
	if keys.HMACOverlap {
		hmac, err := hmacCrypto(jwt, keys)
		if err != nil {
			return tokenMaterial{}, err
		}
		accessV, err := authBaseServices.NewCompositeVerifier(edCrypto.AccessVerifier, hmac.AccessVerifier)
		if err != nil {
			return tokenMaterial{}, err
		}
		refreshV, err := authBaseServices.NewCompositeVerifier(edCrypto.RefreshVerifier, hmac.RefreshVerifier)
		if err != nil {
			return tokenMaterial{}, err
		}
		crypto.AccessVerifier = accessV
		crypto.RefreshVerifier = refreshV
	}
	return tokenMaterial{Crypto: crypto, JWKS: jwks}, nil
}
