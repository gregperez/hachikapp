package lifecycle

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	initModules "github.com/WilsonSayago/initModules/v2"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/core/services"
)

const (
	dispatcherMaxErrorRunes = 300
	dispatcherSendTimeout   = 30 * time.Second
	dispatcherMarkTimeout   = 5 * time.Second
)

type NotificationDispatcherConfig struct {
	Interval  time.Duration
	BatchSize int
	Lease     time.Duration
	BaseURL   string
	Location  *time.Location
}

// NotificationDispatcher periodically claims due email deliveries and sends
// them, retrying along domain.NextDeliveryAttempt. Delivery is at least once.
type NotificationDispatcher struct {
	store  ports.DeliveryPort
	mailer ports.Mailer
	cfg    NotificationDispatcherConfig
	logger *slog.Logger
	now    func() time.Time

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewNotificationDispatcher(
	store ports.DeliveryPort,
	mailer ports.Mailer,
	cfg NotificationDispatcherConfig,
	logger *slog.Logger,
) (*NotificationDispatcher, error) {
	if store == nil {
		return nil, fmt.Errorf("delivery store is nil")
	}
	if mailer == nil {
		return nil, fmt.Errorf("mailer is nil")
	}
	if cfg.Interval <= 0 {
		return nil, fmt.Errorf("notification dispatch interval must be positive")
	}
	if cfg.BatchSize <= 0 {
		return nil, fmt.Errorf("notification dispatch batch size must be positive")
	}
	if cfg.Lease <= 0 {
		return nil, fmt.Errorf("notification dispatch lease must be positive")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &NotificationDispatcher{
		store:  store,
		mailer: mailer,
		cfg:    cfg,
		logger: logger,
		now:    time.Now,
	}, nil
}

// Start launches the loop; the first cycle runs immediately in the goroutine
// so a failing store or mailer never blocks startup.
func (d *NotificationDispatcher) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cancel != nil {
		return fmt.Errorf("notification dispatcher already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	d.cancel = cancel
	d.done = done
	go d.run(runCtx, done)
	return nil
}

// Stop cancels the loop and waits for the in-flight cycle to finish.
func (d *NotificationDispatcher) Stop(ctx context.Context) error {
	d.mu.Lock()
	cancel := d.cancel
	done := d.done
	d.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		d.clearRun(done)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *NotificationDispatcher) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	d.runCycle(ctx)
	ticker := time.NewTicker(d.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.runCycle(ctx)
		}
	}
}

func (d *NotificationDispatcher) runCycle(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	deliveries, err := d.store.ClaimDue(ctx, d.now(), d.cfg.BatchSize, d.cfg.Lease)
	if err != nil {
		if ctx.Err() == nil {
			d.logger.Error("claim notification deliveries failed", "err", err)
		}
		return
	}
	for _, del := range deliveries {
		if ctx.Err() != nil {
			// Unprocessed claims keep their lease and are retried after it expires.
			return
		}
		d.dispatch(ctx, del)
	}
}

func (d *NotificationDispatcher) dispatch(ctx context.Context, del domain.Delivery) {
	email := services.RenderNotificationEmail(del, d.cfg.BaseURL, d.cfg.Location)
	sendCtx, cancel := context.WithTimeout(ctx, dispatcherSendTimeout)
	sendErr := d.mailer.Send(sendCtx, email)
	cancel()

	// Marks use a context detached from shutdown so a completed send is recorded.
	markCtx, markCancel := context.WithTimeout(context.WithoutCancel(ctx), dispatcherMarkTimeout)
	defer markCancel()
	now := d.now()

	if sendErr == nil {
		if err := d.store.MarkSent(markCtx, del.ID, now); err != nil {
			d.logger.Error("mark delivery sent failed", "delivery", del.ID, "err", err)
		}
		return
	}
	if ctx.Err() != nil {
		// Interrupted by shutdown: leave unmarked, the lease expires and it is retried.
		return
	}
	attempts := del.Attempts + 1
	lastErr := truncateRunes(sendErr.Error(), dispatcherMaxErrorRunes)
	next, failed := domain.NextDeliveryAttempt(attempts, now)
	if failed {
		d.logger.Warn("notification delivery failed permanently", "delivery", del.ID, "attempts", attempts)
		if err := d.store.MarkFailed(markCtx, del.ID, attempts, lastErr); err != nil {
			d.logger.Error("mark delivery failed failed", "delivery", del.ID, "err", err)
		}
		return
	}
	d.logger.Warn("notification delivery will be retried", "delivery", del.ID, "attempts", attempts, "next", next)
	if err := d.store.MarkRetry(markCtx, del.ID, attempts, next, lastErr); err != nil {
		d.logger.Error("mark delivery retry failed", "delivery", del.ID, "err", err)
	}
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func (d *NotificationDispatcher) clearRun(done chan struct{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.done == done {
		d.cancel = nil
		d.done = nil
	}
}

var _ initModules.Lifecycle = (*NotificationDispatcher)(nil)
