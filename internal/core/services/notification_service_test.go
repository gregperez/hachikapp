package services

import (
	"context"
	"errors"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

const notifID = "cccccccc-0000-4000-8000-000000000003"

type fakeNotifPort struct {
	markedOwner, markedID string
	markedAt              time.Time
	allOwner              string
	calls                 int
}

func (f *fakeNotifPort) List(context.Context, string, authBaseDomain.PageRequest) (authBaseDomain.Page[domain.Notification], error) {
	f.calls++
	return authBaseDomain.Page[domain.Notification]{}, nil
}
func (f *fakeNotifPort) UnreadCount(context.Context, string) (int, error) { f.calls++; return 3, nil }
func (f *fakeNotifPort) MarkRead(_ context.Context, owner, id string, at time.Time) error {
	f.calls++
	f.markedOwner, f.markedID, f.markedAt = owner, id, at
	return nil
}
func (f *fakeNotifPort) MarkAllRead(_ context.Context, owner string, at time.Time) (int, error) {
	f.calls++
	f.allOwner, f.markedAt = owner, at
	return 2, nil
}

func TestNotificationServiceValidation(t *testing.T) {
	at := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	port := &fakeNotifPort{}
	svc := NewNotificationService(port, func() time.Time { return at })
	ctx := context.Background()

	if err := svc.MarkRead(ctx, ownerA, "not-a-uuid"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("invalid id err=%v", err)
	}
	if port.calls != 0 {
		t.Fatal("invalid id reached the port")
	}
	if err := svc.MarkRead(ctx, " ", notifID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("empty owner err=%v", err)
	}
	if _, err := svc.List(ctx, "", authBaseDomain.PageRequest{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("list empty owner err=%v", err)
	}
	if _, err := svc.UnreadCount(ctx, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("count empty owner err=%v", err)
	}
	if _, err := svc.MarkAllRead(ctx, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("read-all empty owner err=%v", err)
	}
	if port.calls != 0 {
		t.Fatal("empty owner reached the port")
	}
	if err := svc.MarkRead(ctx, ownerA, notifID); err != nil || port.markedOwner != ownerA || port.markedID != notifID || !port.markedAt.Equal(at) {
		t.Fatalf("mark read err=%v port=%+v", err, port)
	}
	if n, err := svc.MarkAllRead(ctx, ownerA); err != nil || n != 2 || port.allOwner != ownerA {
		t.Fatalf("read all n=%d err=%v", n, err)
	}
}
