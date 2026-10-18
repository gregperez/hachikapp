package lifecycle

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/WilsonSayago/base-golang/internal/core/ports"
	initModules "github.com/WilsonSayago/initModules/v2"
)

type AuditEventPurgerConfig struct {
	Interval   time.Duration
	Retention  time.Duration
	MaxBatches int
}

// AuditEventPurger deletes audit rows older than the retention window.
type AuditEventPurger struct {
	store  ports.AuditEventPort
	cfg    AuditEventPurgerConfig
	logger *slog.Logger
	now    func() time.Time

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewAuditEventPurger(
	store ports.AuditEventPort,
	cfg AuditEventPurgerConfig,
	logger *slog.Logger,
) (*AuditEventPurger, error) {
	if store == nil {
		return nil, fmt.Errorf("audit event store is nil")
	}
	if cfg.Interval <= 0 {
		return nil, fmt.Errorf("audit purge interval must be positive")
	}
	if cfg.Retention <= 0 {
		return nil, fmt.Errorf("audit retention must be positive")
	}
	if cfg.MaxBatches <= 0 {
		return nil, fmt.Errorf("audit purge max batches must be positive")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &AuditEventPurger{
		store:  store,
		cfg:    cfg,
		logger: logger,
		now:    time.Now,
	}, nil
}

func (p *AuditEventPurger) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.cancel != nil {
		p.mu.Unlock()
		return fmt.Errorf("audit event purger already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	p.cancel = cancel
	p.done = done
	p.mu.Unlock()

	if err := p.runCycle(runCtx); err != nil {
		cancel()
		p.clearRun(done)
		return fmt.Errorf("initial audit purge: %w", err)
	}
	go p.run(runCtx, done)
	return nil
}

func (p *AuditEventPurger) Stop(ctx context.Context) error {
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

func (p *AuditEventPurger) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.runCycle(ctx); err != nil && ctx.Err() == nil {
				p.logger.Error("audit purge failed", "err", err)
			}
		}
	}
}

func (p *AuditEventPurger) runCycle(ctx context.Context) error {
	before := p.now().UTC().Add(-p.cfg.Retention)
	for i := 0; i < p.cfg.MaxBatches; i++ {
		deleted, err := p.store.PurgeBefore(ctx, before)
		if err != nil {
			return err
		}
		if deleted == 0 {
			return nil
		}
	}
	return nil
}

func (p *AuditEventPurger) clearRun(done chan struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == done {
		p.cancel = nil
		p.done = nil
	}
}

var _ initModules.Lifecycle = (*AuditEventPurger)(nil)
