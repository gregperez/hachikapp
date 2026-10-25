package ports

import (
	"context"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// Mailer sends an email.
type Mailer interface {
	Send(ctx context.Context, email domain.Email) error
}

// NotificationPort reads and updates an owner's in-app notifications.
type NotificationPort interface {
	List(ctx context.Context, userID string, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.Notification], error)
	UnreadCount(ctx context.Context, userID string) (int, error)
	// MarkRead returns domain.ErrNotFound for a missing or foreign notification.
	MarkRead(ctx context.Context, userID, id string, at time.Time) error
	// MarkAllRead returns how many notifications changed.
	MarkAllRead(ctx context.Context, userID string, at time.Time) (int, error)
}

// DeliveryPort drives the email delivery queue.
type DeliveryPort interface {
	// ClaimDue reserves up to limit due deliveries for lease and returns them.
	ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]domain.Delivery, error)
	MarkSent(ctx context.Context, id string, at time.Time) error
	MarkRetry(ctx context.Context, id string, attempts int, next time.Time, lastErr string) error
	MarkFailed(ctx context.Context, id string, attempts int, lastErr string) error
}
