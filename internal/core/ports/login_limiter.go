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

	// PublicTagViewLimit caps anonymous profile and photo views per client IP.
	PublicTagViewLimit LoginLimitScope = "public_tag_view"
	// PublicTagScanLimit caps anonymous scan registrations per client IP.
	PublicTagScanLimit LoginLimitScope = "public_tag_scan"
	// PublicTagMessageLimit caps anonymous messages per client IP and tag.
	PublicTagMessageLimit LoginLimitScope = "public_tag_message"
	// PublicTagContactLimit caps anonymous contact reveals per client IP and tag.
	PublicTagContactLimit LoginLimitScope = "public_tag_contact"
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
