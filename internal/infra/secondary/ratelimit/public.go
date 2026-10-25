package ratelimit

import (
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/ports"
)

// PublicTagScopes returns the fixed per-IP quotas of the anonymous public tag
// routes: profile and photo views, scans, finder messages and contact reveals.
func PublicTagScopes() map[ports.LoginLimitScope]ScopeConfig {
	return map[ports.LoginLimitScope]ScopeConfig{
		ports.PublicTagViewLimit:     {Limit: 60, Window: time.Minute},
		ports.PublicTagScanLimit:     {Limit: 20, Window: time.Hour},
		ports.PublicTagMessageLimit:  {Limit: 5, Window: time.Hour},
		ports.PublicTagContactLimit:  {Limit: 3, Window: time.Hour},
		ports.PublicTagLocationLimit: {Limit: 10, Window: time.Hour},
	}
}
