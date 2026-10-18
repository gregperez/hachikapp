package domain_test

import (
	"testing"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

func TestNormalizeReturnToExactAllowlist(t *testing.T) {
	t.Parallel()
	allowed := []string{"https://app.example.com/auth/oidc"}
	got, err := domain.NormalizeReturnTo("https://app.example.com/auth/oidc", allowed)
	if err != nil || got != allowed[0] {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestNormalizeReturnToRejectsQueryAndUnknownHost(t *testing.T) {
	t.Parallel()
	allowed := []string{"https://app.example.com/auth/oidc"}
	if _, err := domain.NormalizeReturnTo("https://app.example.com/auth/oidc?next=1", allowed); err != domain.ErrOIDCRedirectNotAllowed {
		t.Fatalf("query err=%v", err)
	}
	if _, err := domain.NormalizeReturnTo("https://evil.example/auth/oidc", allowed); err != domain.ErrOIDCRedirectNotAllowed {
		t.Fatalf("host err=%v", err)
	}
}
