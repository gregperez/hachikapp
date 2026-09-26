// Package lifecycle contains background components prepared for application
// lifecycle registration.
package lifecycle

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	authBasePorts "github.com/WilsonSayago/authBase/v4/core/port"
	initModules "github.com/WilsonSayago/initModules/v2"
)

// ExpiredSessionPurgerConfig controls bounded session cleanup.
type ExpiredSessionPurgerConfig struct {
	Interval   time.Duration
	Retention  time.Duration
	MaxBatches int
}

// ExpiredSessionPurger runs retention-aware cleanup at startup and intervals.
// Plan 015 will register it with the process-owned initModules.App.
type ExpiredSessionPurger struct {
	purger authBasePorts.ExpiredSessionPurger
	cfg    ExpiredSessionPurgerConfig
	logger *slog.Logger
	now    func() time.Time

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewExpiredSessionPurger constructs a lifecycle without starting goroutines.
func NewExpiredSessionPurger(
	purger authBasePorts.ExpiredSessionPurger,
	cfg ExpiredSessionPurgerConfig,
	logger *slog.Logger,
) (*ExpiredSessionPurger, error) {
	if purger == nil {
		return nil, fmt.Errorf("expired session purger is nil")
	}
	if cfg.Interval <= 0 {
		return nil, fmt.Errorf("expired session purge interval must be positive")
	}
	if cfg.Retention < 0 {
		return nil, fmt.Errorf("expired session retention must not be negative")
	}
	if cfg.MaxBatches <= 0 {
		return nil, fmt.Errorf("expired session max batches must be positive")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &ExpiredSessionPurger{
		purger: purger,
		cfg:    cfg,
		logger: logger,
		now:    time.Now,
	}, nil
}

// Start runs one bounded cleanup synchronously, then starts interval cleanup.
func (p *ExpiredSessionPurger) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.cancel != nil {
		p.mu.Unlock()
		return fmt.Errorf("expired session purger already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	p.cancel = cancel
	p.done = done
	p.mu.Unlock()

	if err := p.runCycle(runCtx); err != nil {
		cancel()
		p.clearRun(done)
		return fmt.Errorf("initial expired session purge: %w", err)
	}

	go p.run(runCtx, done)
	return nil
}

// Stop cancels interval cleanup and waits for the current batch to finish.
func (p *ExpiredSessionPurger) Stop(ctx context.Context) error {
	p.mu.Lock()
	cancel := p.cancel
	done := p.done
	p.mu.Unlock()
	if cancel == nil {
		return nil
	}

	cancel()
	select {
	case <-done:
		p.clearRun(done)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *ExpiredSessionPurger) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.runCycle(ctx); err != nil && ctx.Err() == nil {
				p.logger.Error("expired session purge failed", "err", err)
			}
		}
	}
}

func (p *ExpiredSessionPurger) runCycle(ctx context.Context) error {
	before := p.now().UTC().Add(-p.cfg.Retention)
	for i := 0; i < p.cfg.MaxBatches; i++ {
		deleted, err := p.purger.PurgeExpired(ctx, before)
		if err != nil {
			return err
		}
		if deleted == 0 {
			return nil
		}
	}
	return nil
}

func (p *ExpiredSessionPurger) clearRun(done chan struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == done {
		p.cancel = nil
		p.done = nil
	}
}

var _ initModules.Lifecycle = (*ExpiredSessionPurger)(nil)
