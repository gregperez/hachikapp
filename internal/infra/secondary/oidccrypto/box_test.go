package oidccrypto_test

import (
	"bytes"
	"testing"

	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/oidccrypto"
)

func TestBoxRoundTrip(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte{0x11}, 32)
	box, err := oidccrypto.NewBox(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal([]byte("pkce-verifier"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := box.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "pkce-verifier" {
		t.Fatalf("plain=%q", plain)
	}
}

func TestBoxRejectsWrongKey(t *testing.T) {
	t.Parallel()
	box, err := oidccrypto.NewBox(bytes.Repeat([]byte{0x11}, 32))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := oidccrypto.NewBox(bytes.Repeat([]byte{0x22}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Open(sealed); err == nil {
		t.Fatal("expected decrypt failure")
	}
}
