package domain

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestNextDeliveryAttemptLadder(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	want := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour}
	for i, d := range want {
		next, failed := NextDeliveryAttempt(i+1, now)
		if failed {
			t.Fatalf("attempt %d: unexpected failed", i+1)
		}
		if !next.Equal(now.Add(d)) {
			t.Fatalf("attempt %d: next = %v, want %v", i+1, next, now.Add(d))
		}
	}
	if _, failed := NextDeliveryAttempt(6, now); !failed {
		t.Fatal("attempt 6 must fail")
	}
	if _, failed := NextDeliveryAttempt(9, now); !failed {
		t.Fatal("attempt 9 must fail")
	}
}

func TestValidateScanLocation(t *testing.T) {
	if err := ValidateScanLocation(-33.45, -70.66, 20); err != nil {
		t.Fatalf("valid location rejected: %v", err)
	}
	cases := []struct {
		name     string
		lat, lng float64
		acc      int
		field    string
	}{
		{"lat high", 91, 0, 0, "lat"},
		{"lat low", -90.5, 0, 0, "lat"},
		{"lng low", 0, -181, 0, "lng"},
		{"lng high", 0, 180.1, 0, "lng"},
		{"acc neg", 0, 0, -1, "accuracy"},
		{"acc high", 0, 0, 100001, "accuracy"},
		{"lat nan", math.NaN(), 0, 0, "lat"},
		{"lng inf", 0, math.Inf(1), 0, "lng"},
		{"lat -inf", math.Inf(-1), 0, 0, "lat"},
	}
	for _, c := range cases {
		err := ValidateScanLocation(c.lat, c.lng, c.acc)
		var pve *PetValidationError
		if !errors.As(err, &pve) {
			t.Fatalf("%s: err = %v, want *PetValidationError", c.name, err)
		}
		if !errors.Is(err, ErrInvalidPet) {
			t.Fatalf("%s: must unwrap to ErrInvalidPet", c.name)
		}
		found := false
		for _, is := range pve.Issues {
			if is.Field == c.field && is.Code == "range" {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: issues = %+v, want %s:range", c.name, pve.Issues, c.field)
		}
	}
	if err := ValidateScanLocation(0, 0, 0); err != nil {
		t.Fatalf("zero values must be valid: %v", err)
	}
	if err := ValidateScanLocation(90, 180, 100000); err != nil {
		t.Fatalf("bounds must be valid: %v", err)
	}
}
