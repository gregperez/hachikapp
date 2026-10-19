package ratelimit

import (
	"testing"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/ports"
)

func TestPublicTagScopesMatchSpec(t *testing.T) {
	want := map[ports.LoginLimitScope]ScopeConfig{
		ports.PublicTagViewLimit:    {Limit: 60, Window: time.Minute},
		ports.PublicTagScanLimit:    {Limit: 20, Window: time.Hour},
		ports.PublicTagMessageLimit: {Limit: 5, Window: time.Hour},
		ports.PublicTagContactLimit: {Limit: 3, Window: time.Hour},
	}
	got := PublicTagScopes()
	if len(got) != len(want) {
		t.Fatalf("scopes=%v", got)
	}
	for scope, policy := range want {
		if got[scope] != policy {
			t.Fatalf("%s=%+v want %+v", scope, got[scope], policy)
		}
	}
	if _, err := NewMemory(Config{Scopes: got, MaxEntries: 1}); err != nil {
		t.Fatal(err)
	}
}
