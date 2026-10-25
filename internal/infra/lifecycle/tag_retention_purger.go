package lifecycle

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	initModules "github.com/WilsonSayago/initModules/v2"
)

// TagRetentionStore is the slice of ports.TagScanPort the purger needs.
type TagRetentionStore interface {
	PurgeTagData(ctx context.Context, now time.Time) (clearedLocations, deletedScans, deletedNotifications int, err error)
}

type TagRetentionPurgerConfig struct {
	Interval time.Duration
}

// TagRetentionPurger enforces the scan-data retention policy. The first run
// happens in the background goroutine so a failing store never blocks startup.
type TagRetentionPurger struct {
	store  TagRetentionStore
	cfg    TagRetentionPurgerConfig
	logger *slog.Logger
	now    func() time.Time

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewTagRetentionPurger(store TagRetentionStore, cfg TagRetentionPurgerConfig, logger *slog.Logger) (*TagRetentionPurger, error) {
	if store == nil {
		return nil, fmt.Errorf("tag retention store is nil")
	}
	if cfg.Interval <= 0 {
		return nil, fmt.Errorf("tag retention interval must be positive")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &TagRetentionPurger{store: store, cfg: cfg, logger: logger, now: time.Now}, nil
}

func (p *TagRetentionPurger) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		return fmt.Errorf("tag retention purger already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	p.cancel = cancel
	p.done = done
	go p.run(runCtx, done)
	return nil
}

func (p *TagRetentionPurger) Stop(ctx context.Context) error {
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

func (p *TagRetentionPurger) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	p.runCycle(ctx)
	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.runCycle(ctx)
		}
	}
}

func (p *TagRetentionPurger) runCycle(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	cleared, scans, notifs, err := p.store.PurgeTagData(ctx, p.now().UTC())
	if err != nil {
		if ctx.Err() == nil {
			p.logger.Error("tag retention purge failed", "err", err,
				"cleared_locations", cleared, "deleted_scans", scans, "deleted_notifications", notifs)
		}
		return
	}
	p.logger.Info("tag retention purge done",
		"cleared_locations", cleared, "deleted_scans", scans, "deleted_notifications", notifs)
}

func (p *TagRetentionPurger) clearRun(done chan struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == done {
		p.cancel = nil
		p.done = nil
	}
}

var _ initModules.Lifecycle = (*TagRetentionPurger)(nil)
