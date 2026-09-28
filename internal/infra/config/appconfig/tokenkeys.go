package appconfig

import "strings"

const (
	TokenAlgHS256   = "hs256"
	TokenAlgEd25519 = "ed25519"
)

// TokenKeysConfig is optional key-agility configuration. HMAC remains default.
type TokenKeysConfig struct {
	Algorithm string

	AccessKID  string
	RefreshKID string

	PreviousAccessKID     string
	PreviousAccessSecret  string
	PreviousRefreshKID    string
	PreviousRefreshSecret string

	AccessKeyPath          string
	RefreshKeyPath         string
	PreviousAccessPubPath  string
	PreviousRefreshPubPath string

	HMACOverlap bool
}

func (c TokenKeysConfig) AlgorithmNorm() string {
	alg := strings.ToLower(strings.TrimSpace(c.Algorithm))
	if alg == "" {
		return TokenAlgHS256
	}
	return alg
}

func (c TokenKeysConfig) validate() error {
	if c.AccessKID != "" && c.AccessKID == c.RefreshKID {
		return RedactedError("auth.tokenKeys.refreshKid", "must differ from access kid")
	}

	switch c.AlgorithmNorm() {
	case TokenAlgHS256:
		if c.AccessKeyPath != "" || c.RefreshKeyPath != "" ||
			c.PreviousAccessPubPath != "" || c.PreviousRefreshPubPath != "" {
			return RedactedError("auth.tokenKeys.algorithm", "ed25519 key paths require algorithm ed25519")
		}
		if c.HMACOverlap {
			return RedactedError("auth.tokenKeys.hmacOverlap", "is only valid with algorithm ed25519")
		}
	case TokenAlgEd25519:
		if strings.TrimSpace(c.AccessKID) == "" {
			return RedactedError("auth.tokenKeys.accessKid", "is required")
		}
		if strings.TrimSpace(c.RefreshKID) == "" {
			return RedactedError("auth.tokenKeys.refreshKid", "is required")
		}
		if strings.TrimSpace(c.AccessKeyPath) == "" {
			return RedactedError("auth.tokenKeys.accessKeyPath", "is required")
		}
		if strings.TrimSpace(c.RefreshKeyPath) == "" {
			return RedactedError("auth.tokenKeys.refreshKeyPath", "is required")
		}
		if c.AccessKeyPath == c.RefreshKeyPath {
			return RedactedError("auth.tokenKeys.refreshKeyPath", "must differ from access key path")
		}
		if c.PreviousAccessPubPath != "" && strings.TrimSpace(c.PreviousAccessKID) == "" {
			return RedactedError("auth.tokenKeys.previousAccessKid", "is required when a previous access public key is set")
		}
		if c.PreviousRefreshPubPath != "" && strings.TrimSpace(c.PreviousRefreshKID) == "" {
			return RedactedError("auth.tokenKeys.previousRefreshKid", "is required when a previous refresh public key is set")
		}
		if c.PreviousAccessKID != "" && (c.PreviousAccessKID == c.AccessKID || c.PreviousAccessKID == c.RefreshKID) {
			return RedactedError("auth.tokenKeys.previousAccessKid", "must be unique")
		}
		if c.PreviousRefreshKID != "" && (c.PreviousRefreshKID == c.AccessKID || c.PreviousRefreshKID == c.RefreshKID || c.PreviousRefreshKID == c.PreviousAccessKID) {
			return RedactedError("auth.tokenKeys.previousRefreshKid", "must be unique")
		}
		if c.PreviousAccessSecret != "" || c.PreviousRefreshSecret != "" {
			return RedactedError("auth.tokenKeys.previousAccessSecret", "hmac previous secrets are only valid with algorithm hs256")
		}
	default:
		return RedactedError("auth.tokenKeys.algorithm", "must be hs256 or ed25519")
	}

	if c.PreviousAccessSecret != "" && strings.TrimSpace(c.PreviousAccessKID) == "" {
		return RedactedError("auth.tokenKeys.previousAccessKid", "is required when a previous access secret is set")
	}
	if c.PreviousRefreshSecret != "" && strings.TrimSpace(c.PreviousRefreshKID) == "" {
		return RedactedError("auth.tokenKeys.previousRefreshKid", "is required when a previous refresh secret is set")
	}
	if c.PreviousAccessSecret != "" && len(c.PreviousAccessSecret) < 32 {
		return RedactedError("auth.tokenKeys.previousAccessSecret", "must be at least 32 bytes")
	}
	if c.PreviousRefreshSecret != "" && len(c.PreviousRefreshSecret) < 32 {
		return RedactedError("auth.tokenKeys.previousRefreshSecret", "must be at least 32 bytes")
	}
	return nil
}
