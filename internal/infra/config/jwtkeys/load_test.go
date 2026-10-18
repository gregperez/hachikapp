package jwtkeys_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregperez/hachikapp-backend/internal/infra/config/jwtkeys"
)

func TestLoadPrivateRejectsOpenPermissions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "open.pem")
	if err := os.WriteFile(path, []byte("not-a-real-key"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := jwtkeys.LoadPrivate(path)
	if err == nil || !strings.Contains(err.Error(), "permissions") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "not-a-real-key") {
		t.Fatalf("leaked material: %v", err)
	}
}

func TestLoadPrivateRoundTrip(t *testing.T) {
	t.Parallel()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "access.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := jwtkeys.LoadPrivate(path)
	if err != nil {
		t.Fatal(err)
	}
	if !priv.Equal(got) {
		t.Fatal("loaded key mismatch")
	}
}

func TestLoadPrivateInvalidDoesNotLeak(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bad.pem")
	payload := []byte("-----BEGIN PRIVATE KEY-----\nSECRETMATERIAL\n-----END PRIVATE KEY-----\n")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := jwtkeys.LoadPrivate(path)
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "SECRETMATERIAL") {
		t.Fatalf("leaked material: %v", err)
	}
}
