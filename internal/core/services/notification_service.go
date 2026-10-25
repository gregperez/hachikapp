package services

import (
	"context"
	"strings"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
)

// NotificationService implements the owner's notification use cases.
type NotificationService struct {
	notifications ports.NotificationPort
	now           func() time.Time
}

func NewNotificationService(notifications ports.NotificationPort, now func() time.Time) *NotificationService {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &NotificationService{notifications: notifications, now: now}
}

var _ core.NotificationUseCase = (*NotificationService)(nil)

func (s *NotificationService) List(ctx context.Context, ownerID string, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.Notification], error) {
	if strings.TrimSpace(ownerID) == "" {
		return authBaseDomain.Page[domain.Notification]{}, domain.ErrForbidden
	}
	return s.notifications.List(ctx, ownerID, page)
}

func (s *NotificationService) UnreadCount(ctx context.Context, ownerID string) (int, error) {
	if strings.TrimSpace(ownerID) == "" {
		return 0, domain.ErrForbidden
	}
	return s.notifications.UnreadCount(ctx, ownerID)
}

func (s *NotificationService) MarkRead(ctx context.Context, ownerID, id string) error {
	if strings.TrimSpace(ownerID) == "" {
		return domain.ErrForbidden
	}
	if !isPetID(id) {
		return domain.ErrNotFound
	}
	return s.notifications.MarkRead(ctx, ownerID, id, s.now())
}

func (s *NotificationService) MarkAllRead(ctx context.Context, ownerID string) (int, error) {
	if strings.TrimSpace(ownerID) == "" {
		return 0, domain.ErrForbidden
	}
	return s.notifications.MarkAllRead(ctx, ownerID, s.now())
}
