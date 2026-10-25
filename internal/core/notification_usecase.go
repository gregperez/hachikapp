package core

import (
	"context"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// NotificationUseCase lets an owner read and clear their in-app notifications.
type NotificationUseCase interface {
	List(ctx context.Context, ownerID string, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.Notification], error)
	UnreadCount(ctx context.Context, ownerID string) (int, error)
	// MarkRead returns domain.ErrNotFound for a missing or foreign notification.
	MarkRead(ctx context.Context, ownerID, id string) error
	MarkAllRead(ctx context.Context, ownerID string) (int, error)
}
