package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/ratelimit"
)

func TestMemoryExactBoundaryResetAndWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	limiter, err := ratelimit.NewMemory(ratelimit.Config{
		Scopes: map[ports.LoginLimitScope]ratelimit.ScopeConfig{
			ports.LoginLimitAccount: {Limit: 2, Window: time.Minute},
		},
		MaxEntries: 2,
		Now:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		result, allowErr := limiter.Allow(context.Background(), ports.LoginLimitAccount, "digest-a")
		if allowErr != nil || !result.Allowed {
			t.Fatalf("attempt %d: result=%+v err=%v", attempt, result, allowErr)
		}
	}
	blocked, err := limiter.Allow(context.Background(), ports.LoginLimitAccount, "digest-a")
	if err != nil || blocked.Allowed || blocked.RetryAfter != time.Minute {
		t.Fatalf("blocked=%+v err=%v", blocked, err)
	}

	if err := limiter.Reset(context.Background(), ports.LoginLimitAccount, "digest-a"); err != nil {
		t.Fatal(err)
	}
	allowed, err := limiter.Allow(context.Background(), ports.LoginLimitAccount, "digest-a")
	if err != nil || !allowed.Allowed {
		t.Fatalf("after reset=%+v err=%v", allowed, err)
	}

	now = now.Add(time.Minute)
	allowed, err = limiter.Allow(context.Background(), ports.LoginLimitAccount, "digest-a")
	if err != nil || !allowed.Allowed {
		t.Fatalf("after window=%+v err=%v", allowed, err)
	}
}

func TestMemoryKeepsKeysAndScopesIndependentAndBounded(t *testing.T) {
	t.Parallel()
	limiter, err := ratelimit.NewMemory(ratelimit.Config{
		Scopes: map[ports.LoginLimitScope]ratelimit.ScopeConfig{
			ports.LoginLimitAccount: {Limit: 1, Window: time.Minute},
			ports.LoginLimitOrigin:  {Limit: 1, Window: time.Minute},
		},
		MaxEntries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, request := range []struct {
		scope ports.LoginLimitScope
		key   string
	}{
		{ports.LoginLimitAccount, "account-a"},
		{ports.LoginLimitAccount, "account-b"},
		{ports.LoginLimitOrigin, "origin-a"},
	} {
		result, allowErr := limiter.Allow(ctx, request.scope, request.key)
		if allowErr != nil || !result.Allowed {
			t.Fatalf("%+v: result=%+v err=%v", request, result, allowErr)
		}
	}
}
