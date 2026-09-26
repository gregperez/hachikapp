package oidccrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// Box encrypts short secrets with AES-256-GCM.
type Box struct {
	gcm cipher.AEAD
}

func NewBox(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("oidc encryption key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("oidc cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("oidc gcm: %w", err)
	}
	return &Box{gcm: gcm}, nil
}

func (b *Box) Seal(plaintext []byte) ([]byte, error) {
	if b == nil || b.gcm == nil {
		return nil, fmt.Errorf("oidc encryption box is nil")
	}
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("oidc nonce: %w", err)
	}
	return b.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (b *Box) Open(ciphertext []byte) ([]byte, error) {
	if b == nil || b.gcm == nil {
		return nil, fmt.Errorf("oidc encryption box is nil")
	}
	size := b.gcm.NonceSize()
	if len(ciphertext) < size {
		return nil, fmt.Errorf("oidc ciphertext is truncated")
	}
	nonce, payload := ciphertext[:size], ciphertext[size:]
	plain, err := b.gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return nil, fmt.Errorf("oidc decrypt: %w", err)
	}
	return plain, nil
}
