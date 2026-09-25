package ports

import (
	"context"
	"time"
)

// LoginLimitScope identifies an independently configured authentication quota.
type LoginLimitScope string

const (
	LoginLimitAccount  LoginLimitScope = "login_account"
	LoginLimitOrigin   LoginLimitScope = "login_origin"
	RefreshLimitFamily LoginLimitScope = "refresh_family"
)

// LoginLimitResult reports whether a request may proceed.
type LoginLimitResult struct {
	Allowed    bool
	RetryAfter time.Duration
}

// LoginLimiter protects authentication work using opaque, pre-hashed keys.
type LoginLimiter interface {
	Allow(ctx context.Context, scope LoginLimitScope, key string) (LoginLimitResult, error)
	Reset(ctx context.Context, scope LoginLimitScope, key string) error
}
