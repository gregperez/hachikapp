package lifecycle

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

type markCall struct {
	kind     string
	id       string
	attempts int
	next     time.Time
	err      string
}

type fakeDeliveries struct {
	mu       sync.Mutex
	batches  [][]domain.Delivery
	claimErr error
	claims   int
	limit    int
	lease    time.Duration
	marks    []markCall
}

func (f *fakeDeliveries) ClaimDue(_ context.Context, _ time.Time, limit int, lease time.Duration) ([]domain.Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claims++
	f.limit, f.lease = limit, lease
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	if len(f.batches) == 0 {
		return nil, nil
	}
	b := f.batches[0]
	f.batches = f.batches[1:]
	return b, nil
}

func (f *fakeDeliveries) MarkSent(_ context.Context, id string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.marks = append(f.marks, markCall{kind: "sent", id: id, next: at})
	return nil
}

func (f *fakeDeliveries) MarkRetry(_ context.Context, id string, attempts int, next time.Time, lastErr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.marks = append(f.marks, markCall{"retry", id, attempts, next, lastErr})
	return nil
}

func (f *fakeDeliveries) MarkFailed(_ context.Context, id string, attempts int, lastErr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.marks = append(f.marks, markCall{kind: "failed", id: id, attempts: attempts, err: lastErr})
	return nil
}

func (f *fakeDeliveries) snapshot() []markCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]markCall(nil), f.marks...)
}

type fakeMailer struct {
	mu   sync.Mutex
	sent []domain.Email
	fn   func(ctx context.Context, e domain.Email) error
}

func (m *fakeMailer) Send(ctx context.Context, e domain.Email) error {
	m.mu.Lock()
	m.sent = append(m.sent, e)
	fn := m.fn
	m.mu.Unlock()
	if fn != nil {
		return fn(ctx, e)
	}
	return nil
}

var dispatcherNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

func newTestDispatcher(t *testing.T, store *fakeDeliveries, m *fakeMailer, interval time.Duration) *NotificationDispatcher {
	t.Helper()
	d, err := NewNotificationDispatcher(store, m, NotificationDispatcherConfig{
		Interval: interval, BatchSize: 20, Lease: domain.DeliveryLease,
		BaseURL: "https://app.example.com", Location: time.UTC,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.now = func() time.Time { return dispatcherNow }
	return d
}

func delivery(id string, attempts int) domain.Delivery {
	return domain.Delivery{
		ID: id, Attempts: attempts, ToEmail: "owner@example.com", OwnerName: "Ana",
		Notification: domain.Notification{ID: "n-" + id, Kind: domain.NotificationTagScanned,
			Payload: domain.NotificationPayload{PetName: "Rex"}},
	}
}

func TestDispatcherSendsAndMarksSent(t *testing.T) {
	store := &fakeDeliveries{batches: [][]domain.Delivery{{delivery("d1", 0), delivery("d2", 0)}}}
	m := &fakeMailer{}
	d := newTestDispatcher(t, store, m, time.Hour)
	d.runCycle(context.Background())
	if store.limit != 20 || store.lease != domain.DeliveryLease {
		t.Fatalf("claim args limit=%d lease=%v", store.limit, store.lease)
	}
	marks := store.snapshot()
	if len(marks) != 2 || marks[0].kind != "sent" || marks[0].id != "d1" || marks[1].id != "d2" || !marks[0].next.Equal(dispatcherNow) {
		t.Fatalf("marks = %+v", marks)
	}
	if len(m.sent) != 2 || m.sent[0].To != "owner@example.com" || !strings.Contains(m.sent[0].Subject, "Rex") {
		t.Fatalf("sent = %+v", m.sent)
	}
}

func TestDispatcherRetryLadder(t *testing.T) {
	want := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour}
	store := &fakeDeliveries{}
	m := &fakeMailer{fn: func(context.Context, domain.Email) error { return errors.New("smtp down") }}
	d := newTestDispatcher(t, store, m, time.Hour)
	for i := 0; i < 6; i++ {
		store.batches = [][]domain.Delivery{{delivery("d1", i)}}
		d.runCycle(context.Background())
	}
	marks := store.snapshot()
	if len(marks) != 6 {
		t.Fatalf("marks = %+v", marks)
	}
	for i, w := range want {
		if marks[i].kind != "retry" || marks[i].attempts != i+1 || !marks[i].next.Equal(dispatcherNow.Add(w)) || marks[i].err != "smtp down" {
			t.Fatalf("mark %d = %+v want +%v", i, marks[i], w)
		}
	}
	if marks[5].kind != "failed" || marks[5].attempts != 6 {
		t.Fatalf("last mark = %+v", marks[5])
	}
}

func TestDispatcherErrorTruncatedTo300(t *testing.T) {
	store := &fakeDeliveries{batches: [][]domain.Delivery{{delivery("d1", 0)}}}
	long := strings.Repeat("é", 500)
	m := &fakeMailer{fn: func(context.Context, domain.Email) error { return errors.New(long) }}
	newTestDispatcher(t, store, m, time.Hour).runCycle(context.Background())
	marks := store.snapshot()
	if len(marks) != 1 || len([]rune(marks[0].err)) != 300 {
		t.Fatalf("marks = %+v", marks)
	}
}

func TestDispatcherOneFailureDoesNotStopOthers(t *testing.T) {
	store := &fakeDeliveries{batches: [][]domain.Delivery{{delivery("d1", 0), delivery("d2", 0)}}}
	calls := 0
	m := &fakeMailer{fn: func(context.Context, domain.Email) error {
		calls++
		if calls == 1 {
			return errors.New("boom")
		}
		return nil
	}}
	newTestDispatcher(t, store, m, time.Hour).runCycle(context.Background())
	marks := store.snapshot()
	if len(marks) != 2 || marks[0].kind != "retry" || marks[1].kind != "sent" {
		t.Fatalf("marks = %+v", marks)
	}
}

func TestDispatcherCancelledMidSendLeavesUnmarked(t *testing.T) {
	store := &fakeDeliveries{batches: [][]domain.Delivery{{delivery("d1", 0), delivery("d2", 0)}}}
	ctx, cancel := context.WithCancel(context.Background())
	m := &fakeMailer{fn: func(c context.Context, _ domain.Email) error {
		cancel()
		<-c.Done()
		return c.Err()
	}}
	newTestDispatcher(t, store, m, time.Hour).runCycle(ctx)
	if marks := store.snapshot(); len(marks) != 0 {
		t.Fatalf("marks = %+v", marks)
	}
	if len(m.sent) != 1 {
		t.Fatalf("second delivery should not be attempted, sent=%d", len(m.sent))
	}
}

func TestDispatcherClaimErrorThenNextCycleRuns(t *testing.T) {
	store := &fakeDeliveries{claimErr: errors.New("db down")}
	m := &fakeMailer{}
	d := newTestDispatcher(t, store, m, time.Hour)
	d.runCycle(context.Background())
	store.mu.Lock()
	store.claimErr = nil
	store.batches = [][]domain.Delivery{{delivery("d1", 0)}}
	store.mu.Unlock()
	d.runCycle(context.Background())
	if marks := store.snapshot(); len(marks) != 1 || marks[0].kind != "sent" {
		t.Fatalf("marks = %+v", marks)
	}
}

func TestDispatcherStopIsGraceful(t *testing.T) {
	store := &fakeDeliveries{batches: [][]domain.Delivery{{delivery("d1", 0)}}}
	started := make(chan struct{})
	release := make(chan struct{})
	m := &fakeMailer{fn: func(context.Context, domain.Email) error {
		close(started)
		<-release
		return nil
	}}
	d := newTestDispatcher(t, store, m, time.Hour)
	if err := d.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := d.Start(context.Background()); err == nil {
		t.Fatal("second Start should fail")
	}
	<-started
	stopped := make(chan error, 1)
	go func() { stopped <- d.Stop(context.Background()) }()
	select {
	case <-stopped:
		t.Fatal("Stop returned before the in-flight cycle finished")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	// the in-flight send completed and was recorded despite the cancellation
	if marks := store.snapshot(); len(marks) != 1 || marks[0].kind != "sent" {
		t.Fatalf("marks = %+v", marks)
	}
	if err := d.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNewNotificationDispatcherValidates(t *testing.T) {
	ok := NotificationDispatcherConfig{Interval: time.Second, BatchSize: 1, Lease: time.Minute}
	if _, err := NewNotificationDispatcher(nil, &fakeMailer{}, ok, nil); err == nil {
		t.Fatal("nil store")
	}
	if _, err := NewNotificationDispatcher(&fakeDeliveries{}, nil, ok, nil); err == nil {
		t.Fatal("nil mailer")
	}
	for _, c := range []NotificationDispatcherConfig{
		{BatchSize: 1, Lease: time.Minute}, {Interval: time.Second, Lease: time.Minute}, {Interval: time.Second, BatchSize: 1},
	} {
		if _, err := NewNotificationDispatcher(&fakeDeliveries{}, &fakeMailer{}, c, nil); err == nil {
			t.Fatalf("cfg %+v should fail", c)
		}
	}
}

func TestDispatcherLeaseBudgetStopsBatch(t *testing.T) {
	store := &fakeDeliveries{batches: [][]domain.Delivery{{delivery("d1", 0), delivery("d2", 0), delivery("d3", 0), delivery("d4", 0)}}}
	m := &fakeMailer{}
	d, err := NewNotificationDispatcher(store, m, NotificationDispatcherConfig{
		Interval: time.Hour, BatchSize: 20, Lease: 2 * time.Minute, SendTimeout: 30 * time.Second, Location: time.UTC,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	clock := dispatcherNow
	d.now = func() time.Time { return clock }
	m.fn = func(context.Context, domain.Email) error { clock = clock.Add(40 * time.Second); return nil }
	d.runCycle(context.Background())
	// elapsed 0 -> send; 40s -> send; 80s -> send; 120s+30s >= lease -> stop.
	marks := store.snapshot()
	if len(m.sent) != 3 || len(marks) != 3 {
		t.Fatalf("sent=%d marks=%+v", len(m.sent), marks)
	}
}

func TestDispatcherPanicIsFailedAttempt(t *testing.T) {
	store := &fakeDeliveries{batches: [][]domain.Delivery{{delivery("d1", 0), delivery("d2", 5)}}}
	m := &fakeMailer{fn: func(context.Context, domain.Email) error { panic(strings.Repeat("x", 400)) }}
	newTestDispatcher(t, store, m, time.Hour).runCycle(context.Background())
	marks := store.snapshot()
	if len(marks) != 2 || marks[0].kind != "retry" || marks[1].kind != "failed" ||
		!strings.HasPrefix(marks[0].err, "panic: ") || len([]rune(marks[0].err)) != 300 {
		t.Fatalf("marks = %+v", marks)
	}
}

func TestNewNotificationDispatcherSendTimeoutBelowLease(t *testing.T) {
	_, err := NewNotificationDispatcher(&fakeDeliveries{}, &fakeMailer{}, NotificationDispatcherConfig{
		Interval: time.Second, BatchSize: 1, Lease: time.Minute, SendTimeout: time.Minute}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
