package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	initModules "github.com/WilsonSayago/initModules/v2"
	"github.com/gregperez/hachikapp-backend/internal/infra/config"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
)

// DatabaseLifecycle pings on start and closes the pool on stop.
type DatabaseLifecycle struct {
	db     *database.PostgresPgx
	logger *slog.Logger
}

func NewDatabaseLifecycle(db *database.PostgresPgx, logger *slog.Logger) *DatabaseLifecycle {
	return &DatabaseLifecycle{db: db, logger: logger}
}

func (l *DatabaseLifecycle) Start(ctx context.Context) error {
	if l == nil || l.db == nil {
		return fmt.Errorf("database lifecycle is not configured")
	}
	if err := l.db.Ping(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}
	return nil
}

func (l *DatabaseLifecycle) Stop(ctx context.Context) error {
	if l != nil && l.db != nil {
		l.db.Close()
	}
	return nil
}

// TranslateLifecycle loads locale files on start.
type TranslateLifecycle struct {
	translate *config.Translate
}

func NewTranslateLifecycle(translate *config.Translate) *TranslateLifecycle {
	return &TranslateLifecycle{translate: translate}
}

func (l *TranslateLifecycle) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if l == nil || l.translate == nil {
		return fmt.Errorf("translate lifecycle is not configured")
	}
	return l.translate.LoadTranslationsFiles()
}

func (l *TranslateLifecycle) Stop(ctx context.Context) error {
	return nil
}

// HTTPLifecycle listens on Start and shuts down with readiness drain on Stop.
type HTTPLifecycle struct {
	server *http.Server
	gate   *ReadyGate
	logger *slog.Logger

	mu    sync.RWMutex
	errCh chan error
	addr  string
}

func NewHTTPLifecycle(server *http.Server, gate *ReadyGate, logger *slog.Logger) *HTTPLifecycle {
	return &HTTPLifecycle{server: server, gate: gate, logger: logger}
}

func (l *HTTPLifecycle) Addr() string {
	if l == nil {
		return ""
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.addr
}

func (l *HTTPLifecycle) Start(ctx context.Context) error {
	if l == nil || l.server == nil {
		return fmt.Errorf("http lifecycle is not configured")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", l.server.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	l.mu.Lock()
	l.addr = ln.Addr().String()
	l.errCh = make(chan error, 1)
	l.mu.Unlock()
	go func() {
		err := l.server.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			l.errCh <- err
			return
		}
		l.errCh <- nil
	}()
	if l.gate != nil {
		l.gate.SetReady(true)
	}
	if l.logger != nil {
		l.logger.Info("http listening", "addr", l.addr)
	}
	return nil
}

func (l *HTTPLifecycle) Stop(ctx context.Context) error {
	if l == nil || l.server == nil {
		return nil
	}
	if l.gate != nil {
		l.gate.SetReady(false)
	}
	if err := l.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if l.errCh == nil {
		return nil
	}
	select {
	case err := <-l.errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Second):
		return fmt.Errorf("http serve did not exit")
	}
}

var (
	_ initModules.Lifecycle = (*DatabaseLifecycle)(nil)
	_ initModules.Lifecycle = (*TranslateLifecycle)(nil)
	_ initModules.Lifecycle = (*HTTPLifecycle)(nil)
)
