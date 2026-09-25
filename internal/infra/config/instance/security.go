package instance

import (
	"fmt"

	"github.com/WilsonSayago/base-golang/internal/core/ports"
	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/ratelimit"
	initModules "github.com/WilsonSayago/initModules/v2"
)

func LoginLimiterInstance(cfg appconfig.AuthConfig) ports.LoginLimiter {
	return initModules.Once(func() *ratelimit.Memory {
		limiter, err := ratelimit.NewMemory(ratelimit.Config{
			Scopes: map[ports.LoginLimitScope]ratelimit.ScopeConfig{
				ports.LoginLimitAccount: {
					Limit:  cfg.LoginAccountLimit,
					Window: cfg.RateLimitWindow,
				},
				ports.LoginLimitOrigin: {
					Limit:  cfg.LoginOriginLimit,
					Window: cfg.RateLimitWindow,
				},
				ports.RefreshLimitFamily: {
					Limit:  cfg.RefreshFamilyLimit,
					Window: cfg.RateLimitWindow,
				},
			},
			MaxEntries: cfg.RateLimitMaxEntries,
		})
		if err != nil {
			panic(fmt.Errorf("login limiter: %w", err))
		}
		return limiter
	})
}

func ClientOriginResolverInstance(cfg appconfig.AuthConfig) *controller.ClientOriginResolver {
	return initModules.Once(func() *controller.ClientOriginResolver {
		resolver, err := controller.NewClientOriginResolver(cfg.TrustedProxyCIDRs)
		if err != nil {
			panic(fmt.Errorf("client origin resolver: %w", err))
		}
		return resolver
	})
}
