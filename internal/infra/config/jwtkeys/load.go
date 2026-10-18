package jwtkeys

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// LoadPrivate reads an owner-only Ed25519 private key (PKCS#8 PEM, 32-byte seed, or 64-byte key).
// Errors never include file contents.
func LoadPrivate(path string) (ed25519.PrivateKey, error) {
	raw, err := readOwnerOnly(path)
	if err != nil {
		return nil, err
	}
	if block, _ := pem.Decode(raw); block != nil {
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("jwt key file is invalid")
		}
		key, ok := parsed.(ed25519.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("jwt key file is invalid")
		}
		return key, nil
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	default:
		return nil, fmt.Errorf("jwt key file is invalid")
	}
}

// LoadPublic reads an owner-only Ed25519 public key (PKIX PEM or 32-byte raw).
func LoadPublic(path string) (ed25519.PublicKey, error) {
	raw, err := readOwnerOnly(path)
	if err != nil {
		return nil, err
	}
	if block, _ := pem.Decode(raw); block != nil {
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("jwt key file is invalid")
		}
		key, ok := parsed.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("jwt key file is invalid")
		}
		return key, nil
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("jwt key file is invalid")
	}
	return ed25519.PublicKey(raw), nil
}

func readOwnerOnly(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("jwt key path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("jwt key file is not readable")
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("jwt key file is invalid")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("jwt key file permissions are too open")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jwt key file is not readable")
	}
	return raw, nil
}
