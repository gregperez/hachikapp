package lifecycle

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeRetentionStore struct {
	mu    sync.Mutex
	calls []time.Time
	err   error
	ch    chan struct{}
}

func (f *fakeRetentionStore) PurgeTagData(_ context.Context, now time.Time) (int, int, int, error) {
	f.mu.Lock()
	f.calls = append(f.calls, now)
	err := f.err
	f.mu.Unlock()
	select {
	case f.ch <- struct{}{}:
	default:
	}
	return 2, 3, 4, err
}

func TestNewTagRetentionPurgerValidates(t *testing.T) {
	if _, err := NewTagRetentionPurger(nil, TagRetentionPurgerConfig{Interval: time.Hour}, nil); err == nil {
		t.Fatal("nil store must fail")
	}
	if _, err := NewTagRetentionPurger(&fakeRetentionStore{}, TagRetentionPurgerConfig{}, nil); err == nil {
		t.Fatal("zero interval must fail")
	}
}

func TestTagRetentionPurgerCycleUsesClock(t *testing.T) {
	store := &fakeRetentionStore{}
	p, err := NewTagRetentionPurger(store, TagRetentionPurgerConfig{Interval: time.Hour}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return fixed }
	p.runCycle(context.Background())
	if len(store.calls) != 1 || !store.calls[0].Equal(fixed) {
		t.Fatalf("calls=%v", store.calls)
	}
	store.err = errors.New("boom")
	p.runCycle(context.Background()) // must log, not panic
	if len(store.calls) != 2 {
		t.Fatalf("calls=%v", store.calls)
	}
}

func TestTagRetentionPurgerStartRunsInBackgroundAndStops(t *testing.T) {
	store := &fakeRetentionStore{ch: make(chan struct{}, 4), err: errors.New("down")}
	p, err := NewTagRetentionPurger(store, TagRetentionPurgerConfig{Interval: 10 * time.Millisecond}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("start must not fail on a failing store: %v", err)
	}
	if err := p.Start(context.Background()); err == nil {
		t.Fatal("second start must fail")
	}
	for i := 0; i < 2; i++ {
		select {
		case <-store.ch:
		case <-time.After(2 * time.Second):
			t.Fatal("purger did not run")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}
