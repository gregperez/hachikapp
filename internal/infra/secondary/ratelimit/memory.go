package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/WilsonSayago/base-golang/internal/core/ports"
)

type ScopeConfig struct {
	Limit  int
	Window time.Duration
}

type Config struct {
	Scopes     map[ports.LoginLimitScope]ScopeConfig
	MaxEntries int
	Now        func() time.Time
}

type bucket struct {
	count     int
	expiresAt time.Time
}

// Memory is a bounded fixed-window limiter for single-replica deployments.
type Memory struct {
	mu         sync.Mutex
	scopes     map[ports.LoginLimitScope]ScopeConfig
	maxEntries int
	now        func() time.Time
	buckets    map[string]bucket
}

func NewMemory(cfg Config) (*Memory, error) {
	if cfg.MaxEntries <= 0 {
		return nil, fmt.Errorf("rate limiter max entries must be positive")
	}
	for scope, policy := range cfg.Scopes {
		if policy.Limit <= 0 || policy.Window <= 0 {
			return nil, fmt.Errorf("rate limiter scope %q has invalid policy", scope)
		}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Memory{
		scopes:     cfg.Scopes,
		maxEntries: cfg.MaxEntries,
		now:        now,
		buckets:    make(map[string]bucket),
	}, nil
}

func (m *Memory) Allow(
	ctx context.Context,
	scope ports.LoginLimitScope,
	key string,
) (ports.LoginLimitResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.LoginLimitResult{}, err
	}
	policy, ok := m.scopes[scope]
	if !ok {
		return ports.LoginLimitResult{}, fmt.Errorf("rate limiter scope %q is not configured", scope)
	}
	now := m.now()
	mapKey := string(scope) + ":" + key

	m.mu.Lock()
	defer m.mu.Unlock()

	m.pruneExpired(now)
	current, exists := m.buckets[mapKey]
	if !exists {
		m.makeRoom()
		current = bucket{expiresAt: now.Add(policy.Window)}
	}
	if current.count >= policy.Limit {
		return ports.LoginLimitResult{
			Allowed:    false,
			RetryAfter: max(current.expiresAt.Sub(now), time.Second),
		}, nil
	}
	current.count++
	m.buckets[mapKey] = current
	return ports.LoginLimitResult{Allowed: true}, nil
}

func (m *Memory) Reset(ctx context.Context, scope ports.LoginLimitScope, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.buckets, string(scope)+":"+key)
	m.mu.Unlock()
	return nil
}

func (m *Memory) pruneExpired(now time.Time) {
	for key, value := range m.buckets {
		if !value.expiresAt.After(now) {
			delete(m.buckets, key)
		}
	}
}

func (m *Memory) makeRoom() {
	if len(m.buckets) < m.maxEntries {
		return
	}
	var oldestKey string
	var oldestExpiry time.Time
	for key, value := range m.buckets {
		if oldestKey == "" || value.expiresAt.Before(oldestExpiry) {
			oldestKey = key
			oldestExpiry = value.expiresAt
		}
	}
	delete(m.buckets, oldestKey)
}
