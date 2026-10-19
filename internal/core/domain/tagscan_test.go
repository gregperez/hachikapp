package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestIPPrefix(t *testing.T) {
	cases := map[string]string{
		"203.0.113.45":       "203.0.113.0/24",
		"2001:db8::1":        "2001:db8::/48",
		"2001:db8:1:2:3::9":  "2001:db8:1::/48",
		"::ffff:203.0.113.9": "203.0.113.0/24",
		"not-an-ip":          "",
		"":                   "",
	}
	for in, want := range cases {
		if got := IPPrefix(in); got != want {
			t.Errorf("IPPrefix(%q)=%q want %q", in, got, want)
		}
	}
}

func TestKeyedHashDependsOnPepper(t *testing.T) {
	a := KeyedHash([]byte("pepper-a"), "visitor")
	b := KeyedHash([]byte("pepper-b"), "visitor")
	if a == b {
		t.Fatal("hash must depend on pepper")
	}
	if len(a) != 64 || a != KeyedHash([]byte("pepper-a"), "visitor") {
		t.Fatalf("hash=%q", a)
	}
}

func TestNewScanToken(t *testing.T) {
	tok, err := NewScanToken(strings.NewReader(strings.Repeat("x", 32)))
	if err != nil || len(tok) != 43 {
		t.Fatalf("tok=%q err=%v", tok, err)
	}
	if _, err := NewScanToken(strings.NewReader("short")); err == nil {
		t.Fatal("expected error on short reader")
	}
}

func TestUserAgentFamily(t *testing.T) {
	cases := map[string]string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)": "ios",
		"Mozilla/5.0 (iPad; CPU OS 17_0)":                        "ios",
		"Mozilla/5.0 (Linux; Android 14; Pixel 8)":               "android",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64)":              "desktop",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)":        "desktop",
		"Mozilla/5.0 (X11; Linux x86_64)":                        "desktop",
		"curl/8.0":                                               "other",
		"":                                                       "other",
	}
	for in, want := range cases {
		if got := UserAgentFamily(in); got != want {
			t.Errorf("UserAgentFamily(%q)=%q want %q", in, got, want)
		}
	}
}

func TestNormalizeFinderMessage(t *testing.T) {
	got := NormalizeFinderMessage(FinderMessageInput{
		Body:        "  hola\r\n  encontré   a tu perro \r\n",
		FinderName:  "  Ana   Pérez ",
		FinderPhone: " +56  9 1234   5678 ",
	})
	if got.Body != "hola\n  encontré   a tu perro" || got.FinderName != "Ana Pérez" || got.FinderPhone != "+56 9 1234 5678" {
		t.Fatalf("got=%+v", got)
	}
}

func TestValidateFinderMessage(t *testing.T) {
	cases := []struct {
		name        string
		in          FinderMessageInput
		field, code string
	}{
		{"empty body", FinderMessageInput{}, "body", "required"},
		{"long body", FinderMessageInput{Body: strings.Repeat("a", 501)}, "body", "max"},
		{"long name", FinderMessageInput{Body: "x", FinderName: strings.Repeat("a", 81)}, "finderName", "max"},
		{"bad phone", FinderMessageInput{Body: "x", FinderPhone: "123"}, "finderPhone", "phone"},
	}
	for _, tc := range cases {
		err := ValidateFinderMessage(tc.in)
		var pve *PetValidationError
		if !errors.As(err, &pve) || !errors.Is(err, ErrInvalidPet) || len(pve.Issues) != 1 ||
			pve.Issues[0].Field != tc.field || pve.Issues[0].Code != tc.code {
			t.Errorf("%s: err=%v", tc.name, err)
		}
	}
	if err := ValidateFinderMessage(FinderMessageInput{Body: strings.Repeat("ñ", 500)}); err != nil {
		t.Fatalf("500 runes: %v", err)
	}
	if err := ValidateFinderMessage(FinderMessageInput{Body: "x", FinderName: "Ana", FinderPhone: "+56 9 1234 5678"}); err != nil {
		t.Fatal(err)
	}
}
