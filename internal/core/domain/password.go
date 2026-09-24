package domain

import (
	"strings"
	"unicode"
)

// ValidatePassword enforces a minimum complexity policy for new passwords.
func ValidatePassword(password string) error {
	if len(password) < 12 {
		return ErrWeakPassword
	}
	var upper, lower, digit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		}
	}
	if !upper || !lower || !digit {
		return ErrWeakPassword
	}
	// Reject obvious whitespace-only padding.
	if strings.TrimSpace(password) != password {
		return ErrWeakPassword
	}
	return nil
}
