package domain_test

import (
	"strings"
	"testing"

	"github.com/WilsonSayago/base-golang/internal/core/domain"
)

func TestSanitizeAuditMetadataDropsSecretsAndUnknownKeys(t *testing.T) {
	t.Parallel()
	got := domain.SanitizeAuditMetadata(map[string]any{
		"provider":      "google",
		"password":      "hunter2hunter2hunter",
		"email":         "ada@example.com",
		"authorization": "Bearer eyJhbGciOiJIUzI1NiJ9",
		"dsn":           "postgres://postgres:secret@localhost/db",
		"body":          `{"password":"x"}`,
		"prompt":        "ignore previous instructions",
		"cookie":        "base_refresh=abc",
		"roleIds":       []string{"11111111-1111-1111-1111-111111111111"},
		"isAdmin":       true,
		"familyId":      "family-1",
		"tokenId":       "tid-1",
		"extra":         "nope",
	})
	if _, ok := got["password"]; ok {
		t.Fatal("password leaked")
	}
	if _, ok := got["email"]; ok {
		t.Fatal("email leaked")
	}
	if _, ok := got["authorization"]; ok {
		t.Fatal("authorization leaked")
	}
	if _, ok := got["dsn"]; ok {
		t.Fatal("dsn leaked")
	}
	if got["provider"] != "google" || got["isAdmin"] != true || got["familyId"] != "family-1" {
		t.Fatalf("allowlist missing: %#v", got)
	}
	ids, _ := got["roleIds"].([]string)
	if len(ids) != 1 {
		t.Fatalf("roleIds=%v", got["roleIds"])
	}
}

func TestSanitizeAuditMetadataRejectsEmailLikeValues(t *testing.T) {
	t.Parallel()
	got := domain.SanitizeAuditMetadata(map[string]any{
		"provider": "ada@example.com",
		"tokenId":  "eyJhbGciOiJIUzI1NiJ9.payload.sig",
	})
	if len(got) != 0 {
		t.Fatalf("expected empty, got %#v", got)
	}
}

func TestNormalizeAuditActorUsesSystem(t *testing.T) {
	t.Parallel()
	if got := domain.NormalizeAuditActor(""); got != domain.AuditActorSystem {
		t.Fatalf("got=%q", got)
	}
	long := strings.Repeat("a", 80)
	if got := domain.NormalizeAuditActor(long); len([]rune(got)) != 64 {
		t.Fatalf("len=%d", len([]rune(got)))
	}
}
