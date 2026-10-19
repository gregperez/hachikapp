package domain

import (
	"crypto/rand"
	"errors"
	"strings"
	"testing"
)

func TestNewTagCodeUsesCrockfordAlphabet(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		code, err := NewTagCode(rand.Reader)
		if err != nil {
			t.Fatalf("NewTagCode: %v", err)
		}
		if len(code) != TagCodeLength {
			t.Fatalf("code %q length = %d, want %d", code, len(code), TagCodeLength)
		}
		for _, c := range code {
			if !strings.ContainsRune(TagCodeAlphabet, c) {
				t.Fatalf("code %q has char %q outside alphabet", code, c)
			}
		}
		if _, dup := seen[code]; dup {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = struct{}{}
	}
}

func TestNormalizeTagCode(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"k7m2-q9xr 4p", "K7M2Q9XR4P", true},
		{"K7M2Q9XR4O", "K7M2Q9XR40", true},
		{"K7M2Q9XR4I", "K7M2Q9XR41", true},
		{"K7M2Q9XR4L", "K7M2Q9XR41", true},
		{"ABC", "", false},
		{"K7M2Q9XR4U", "", false},
	}
	for _, tt := range tests {
		got, ok := NormalizeTagCode(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("NormalizeTagCode(%q) = (%q,%v), want (%q,%v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestCheckActivatableBy(t *testing.T) {
	tests := []struct {
		name string
		tag  PetTag
		want error
	}{
		{"unassigned no owner", PetTag{Status: TagStatusUnassigned}, nil},
		{"unassigned same owner", PetTag{Status: TagStatusUnassigned, OwnerID: "u1"}, nil},
		{"unassigned other owner", PetTag{Status: TagStatusUnassigned, OwnerID: "u2"}, ErrTagNotAssignable},
		{"active", PetTag{Status: TagStatusActive, OwnerID: "u1"}, ErrTagNotAssignable},
		{"disabled", PetTag{Status: TagStatusDisabled, OwnerID: "u1"}, ErrTagNotAssignable},
	}
	for _, tt := range tests {
		if got := tt.tag.CheckActivatableBy("u1"); !errors.Is(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestValidateTagBatch(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		quantity int
		field    string
		code     string
	}{
		{"quantity zero", "Lote A", 0, "quantity", "range"},
		{"quantity too big", "Lote A", 1001, "quantity", "range"},
		{"empty label", "   ", 10, "label", "required"},
		{"long label", strings.Repeat("a", 81), 10, "label", "max"},
	}
	for _, tt := range tests {
		err := ValidateTagBatch(tt.label, tt.quantity)
		var pve *PetValidationError
		if !errors.As(err, &pve) {
			t.Fatalf("%s: err = %v, want *PetValidationError", tt.name, err)
		}
		if len(pve.Issues) != 1 || pve.Issues[0].Field != tt.field || pve.Issues[0].Code != tt.code {
			t.Errorf("%s: issues = %+v, want %s/%s", tt.name, pve.Issues, tt.field, tt.code)
		}
	}
	if err := ValidateTagBatch("Lote A", 1000); err != nil {
		t.Errorf("valid batch: %v", err)
	}
	if err := ValidateTagBatch(strings.Repeat("ñ", 80), 1); err != nil {
		t.Errorf("80-rune label should be valid: %v", err)
	}
	var pve *PetValidationError
	if !errors.As(ValidateTagBatch("", 0), &pve) || len(pve.Issues) != 2 {
		t.Errorf("expected both issues, got %v", pve)
	}
}
