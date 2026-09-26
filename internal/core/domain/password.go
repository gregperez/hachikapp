package domain

import (
	"context"
	"unicode/utf8"
)

const (
	MinPasswordRunes = 15
	MaxPasswordRunes = 128
	MaxPasswordBytes = 1024
)

// PasswordBlocklist checks passwords against an offline, locally configured set.
type PasswordBlocklist interface {
	Blocked(ctx context.Context, password string) (bool, error)
}

// ValidatePassword enforces the length policy without a blocklist.
func ValidatePassword(password string) error {
	return ValidatePasswordContext(context.Background(), password, nil)
}

// ValidatePasswordContext enforces the password policy and optional contextual checks.
func ValidatePasswordContext(
	ctx context.Context,
	password string,
	blocklist PasswordBlocklist,
	contextualTerms ...string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	runes := utf8.RuneCountInString(password)
	if !utf8.ValidString(password) ||
		runes < MinPasswordRunes ||
		runes > MaxPasswordRunes ||
		len(password) > MaxPasswordBytes {
		return ErrWeakPassword
	}
	if containsContextualTerm(password, contextualTerms) {
		return ErrWeakPassword
	}
	if blocklist == nil {
		return nil
	}
	blocked, err := blocklist.Blocked(ctx, password)
	if err != nil {
		return err
	}
	if blocked {
		return ErrWeakPassword
	}
	return nil
}
