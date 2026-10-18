package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/WilsonSayago/base-golang/internal/core/domain"
)

func TestDefaultAppSettings(t *testing.T) {
	t.Parallel()
	got := domain.DefaultAppSettings()
	if got.DisplayName != "Base Console" || got.DefaultLocale != "es" || got.Version != 1 {
		t.Fatalf("defaults=%+v", got)
	}
	pub := domain.PublicView(got)
	if pub.DisplayName != got.DisplayName || pub.LogoURL != "" {
		t.Fatalf("public=%+v", pub)
	}
}

func TestValidateAppSettings(t *testing.T) {
	t.Parallel()
	valid := domain.DefaultAppSettings()
	if err := domain.ValidateAppSettings(valid); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		mut  func(*domain.AppSettings)
		want error
		frag string
	}{
		{"empty name", func(s *domain.AppSettings) { s.DisplayName = "   " }, domain.ErrInvalidSettings, "displayName"},
		{"long name", func(s *domain.AppSettings) { s.DisplayName = strings.Repeat("a", 81) }, domain.ErrInvalidSettings, "displayName"},
		{"bad locale", func(s *domain.AppSettings) { s.DefaultLocale = "fr" }, domain.ErrInvalidSettings, "defaultLocale"},
		{"js logo", func(s *domain.AppSettings) { s.LogoURL = "javascript:alert(1)" }, domain.ErrInvalidSettings, "logoUrl"},
		{"http logo", func(s *domain.AppSettings) { s.LogoURL = "http://example.com/logo.png" }, domain.ErrInvalidSettings, "logoUrl"},
		{"protocol-relative", func(s *domain.AppSettings) { s.LogoURL = "//cdn.example/logo.png" }, domain.ErrInvalidSettings, "logoUrl"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := domain.DefaultAppSettings()
			tc.mut(&s)
			err := domain.ValidateAppSettings(s)
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.frag) {
				t.Fatalf("err=%v want %v containing %q", err, tc.want, tc.frag)
			}
		})
	}
	ok := domain.DefaultAppSettings()
	ok.LogoURL = "https://cdn.example.com/logo.png"
	if err := domain.ValidateAppSettings(ok); err != nil {
		t.Fatal(err)
	}
	ok.LogoURL = "/branding/logo.svg"
	if err := domain.ValidateAppSettings(ok); err != nil {
		t.Fatal(err)
	}
}

func TestSettingKeyAllowlist(t *testing.T) {
	t.Parallel()
	if !domain.IsAllowedSettingKey("displayName") || domain.IsAllowedSettingKey("jwtSecret") {
		t.Fatal("allowlist")
	}
	if !domain.IsProtectedSettingKey("jwtSecret") || !domain.IsProtectedSettingKey("databaseUrl") || !domain.IsProtectedSettingKey("corsOrigins") {
		t.Fatal("protected keys")
	}
	if domain.IsProtectedSettingKey("displayName") {
		t.Fatal("displayName must not be treated as protected")
	}
}

func TestChangedSettingFields(t *testing.T) {
	t.Parallel()
	before := domain.DefaultAppSettings()
	after := before
	after.DisplayName = "Acme"
	after.DefaultLocale = "en"
	got := domain.ChangedSettingFields(before, after)
	if len(got) != 2 || got[0] != "displayName" || got[1] != "defaultLocale" {
		t.Fatalf("fields=%v", got)
	}
}
