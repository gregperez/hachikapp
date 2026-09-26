package lifecycle

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeExpiredSessionStore struct {
	mu      sync.Mutex
	results []int64
	err     error
	calls   chan time.Time
}

func (f *fakeExpiredSessionStore) PurgeExpired(ctx context.Context, before time.Time) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls != nil {
		select {
		case f.calls <- before:
		default:
		}
	}
	if f.err != nil {
		return 0, f.err
	}
	if len(f.results) == 0 {
		return 0, nil
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result, nil
}

func TestExpiredSessionPurgerRunsAtStartAndInterval(t *testing.T) {
	t.Parallel()

	calls := make(chan time.Time, 4)
	store := &fakeExpiredSessionStore{calls: calls}
	worker, err := NewExpiredSessionPurger(store, ExpiredSessionPurgerConfig{
		Interval:   10 * time.Millisecond,
		Retention:  time.Hour,
		MaxBatches: 1,
	}, nil)
	if err != nil {
		t.Fatalf("NewExpiredSessionPurger() error = %v", err)
	}
	fixedNow := time.Date(2026, 9, 25, 19, 0, 0, 0, time.UTC)
	worker.now = func() time.Time { return fixedNow }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	for i := 0; i < 2; i++ {
		select {
		case before := <-calls:
			if !before.Equal(fixedNow.Add(-time.Hour)) {
				t.Fatalf("cutoff = %v, want %v", before, fixedNow.Add(-time.Hour))
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for purge")
		}
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := worker.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestExpiredSessionPurgerBoundsStartupBatches(t *testing.T) {
	t.Parallel()

	calls := make(chan time.Time, 4)
	store := &fakeExpiredSessionStore{
		results: []int64{1, 1, 1},
		calls:   calls,
	}
	worker, err := NewExpiredSessionPurger(store, ExpiredSessionPurgerConfig{
		Interval:   time.Hour,
		MaxBatches: 2,
	}, nil)
	if err != nil {
		t.Fatalf("NewExpiredSessionPurger() error = %v", err)
	}
	if err := worker.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if got := len(calls); got != 2 {
		t.Fatalf("startup purge calls = %d, want 2", got)
	}
	if err := worker.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestExpiredSessionPurgerPropagatesInitialFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("purge unavailable")
	worker, err := NewExpiredSessionPurger(
		&fakeExpiredSessionStore{err: sentinel},
		ExpiredSessionPurgerConfig{Interval: time.Hour, MaxBatches: 1},
		nil,
	)
	if err != nil {
		t.Fatalf("NewExpiredSessionPurger() error = %v", err)
	}
	if err := worker.Start(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("Start() error = %v, want sentinel", err)
	}
}

func TestExpiredSessionPurgerValidatesConfiguration(t *testing.T) {
	t.Parallel()

	store := &fakeExpiredSessionStore{}
	tests := []ExpiredSessionPurgerConfig{
		{Interval: 0, MaxBatches: 1},
		{Interval: time.Hour, Retention: -time.Second, MaxBatches: 1},
		{Interval: time.Hour, MaxBatches: 0},
	}
	for _, cfg := range tests {
		if worker, err := NewExpiredSessionPurger(store, cfg, nil); err == nil || worker != nil {
			t.Fatalf("NewExpiredSessionPurger(%+v) = %#v, %v; want validation error", cfg, worker, err)
		}
	}
}
