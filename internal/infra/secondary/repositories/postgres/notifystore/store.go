// Package notifystore implements the in-app notification and email delivery
// ports on PostgreSQL.
package notifystore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	defaultPageSize           = 20
	maxLastErrorRunes         = 300
	invalidTextRepresentation = "22P02"

	notificationColumns = `n.id::text, n.user_id::text, n.pet_id::text, n.tag_id::text,
COALESCE(n.scan_id::text, ''), COALESCE(n.message_id::text, ''), n.kind, n.payload, n.read_at, n.created_at`
)

// Store persists notifications and their email deliveries.
type Store struct {
	db *database.PostgresPgx
}

func New(db *database.PostgresPgx) *Store {
	return &Store{db: db}
}

var (
	_ ports.NotificationPort = (*Store)(nil)
	_ ports.DeliveryPort     = (*Store)(nil)
)

func (s *Store) List(ctx context.Context, userID string, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.Notification], error) {
	limit := page.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	var (
		rows pgx.Rows
		err  error
	)
	if page.After == nil {
		rows, err = s.db.Query(ctx, `SELECT `+notificationColumns+`
FROM public.notifications n
WHERE n.user_id = $1::uuid
ORDER BY n.created_at DESC, n.id DESC
LIMIT $2`, userID, limit+1)
	} else {
		rows, err = s.db.Query(ctx, `SELECT `+notificationColumns+`
FROM public.notifications n
WHERE n.user_id = $1::uuid AND (n.created_at, n.id) < ($2, $3::uuid)
ORDER BY n.created_at DESC, n.id DESC
LIMIT $4`, userID, page.After.CreatedAt, page.After.ID, limit+1)
	}
	if err != nil {
		return authBaseDomain.Page[domain.Notification]{}, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()
	items := make([]domain.Notification, 0, limit)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return authBaseDomain.Page[domain.Notification]{}, fmt.Errorf("scan notification: %w", err)
		}
		items = append(items, n)
	}
	if err := rows.Err(); err != nil {
		return authBaseDomain.Page[domain.Notification]{}, fmt.Errorf("list notifications: %w", err)
	}
	hasNext := len(items) > limit
	if hasNext {
		items = items[:limit]
	}
	return authBaseDomain.Page[domain.Notification]{Items: items, HasNext: hasNext}, nil
}

func (s *Store) UnreadCount(ctx context.Context, userID string) (int, error) {
	var n int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM public.notifications WHERE user_id = $1::uuid AND read_at IS NULL`, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return n, nil
}

// MarkRead is idempotent: an already read notification keeps its first read time.
func (s *Store) MarkRead(ctx context.Context, userID, id string, at time.Time) error {
	tag, err := s.db.Exec(ctx, `UPDATE public.notifications SET read_at = COALESCE(read_at, $3)
WHERE id = $1::uuid AND user_id = $2::uuid`, id, userID, at)
	if err != nil {
		if isInvalidText(err) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("mark notification read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) MarkAllRead(ctx context.Context, userID string, at time.Time) (int, error) {
	tag, err := s.db.Exec(ctx, `UPDATE public.notifications SET read_at = $2
WHERE user_id = $1::uuid AND read_at IS NULL`, userID, at)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// ClaimDue atomically reserves due deliveries (FOR UPDATE SKIP LOCKED) by
// moving next_attempt_at to now+lease, so concurrent workers never share one
// and a crashed worker's deliveries come back after the lease. Attempts is the
// value before this claim.
func (s *Store) ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]domain.Delivery, error) {
	if limit <= 0 {
		return nil, nil
	}
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim deliveries: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	rows, err := tx.Query(ctx, `WITH due AS (
    SELECT id FROM public.notification_deliveries
    WHERE status = 'pending' AND next_attempt_at <= $1
    ORDER BY next_attempt_at
    LIMIT $2
    FOR UPDATE SKIP LOCKED
), claimed AS (
    UPDATE public.notification_deliveries d SET next_attempt_at = $3
    FROM due WHERE d.id = due.id
    RETURNING d.id, d.notification_id, d.attempts
)
SELECT c.id::text, c.notification_id::text, c.attempts, `+notificationColumns+`, u.email, u.name
FROM claimed c
JOIN public.notifications n ON n.id = c.notification_id
JOIN public.users u ON u.id = n.user_id
ORDER BY n.created_at, c.id`, now, limit, now.Add(lease))
	if err != nil {
		return nil, fmt.Errorf("claim deliveries: %w", err)
	}
	deliveries := make([]domain.Delivery, 0, limit)
	for rows.Next() {
		var d domain.Delivery
		n, err := scanClaimed(rows, &d)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan delivery: %w", err)
		}
		d.Notification = n
		deliveries = append(deliveries, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim deliveries: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim deliveries: %w", err)
	}
	committed = true
	return deliveries, nil
}

func (s *Store) MarkSent(ctx context.Context, id string, at time.Time) error {
	return s.updateDelivery(ctx, "mark delivery sent", `UPDATE public.notification_deliveries
SET status = 'sent', sent_at = $2 WHERE id = $1::uuid`, id, at)
}

func (s *Store) MarkRetry(ctx context.Context, id string, attempts int, next time.Time, lastErr string) error {
	return s.updateDelivery(ctx, "mark delivery retry", `UPDATE public.notification_deliveries
SET attempts = $2, next_attempt_at = $3, last_error = $4 WHERE id = $1::uuid`, id, attempts, next, truncate(lastErr))
}

func (s *Store) MarkFailed(ctx context.Context, id string, attempts int, lastErr string) error {
	return s.updateDelivery(ctx, "mark delivery failed", `UPDATE public.notification_deliveries
SET status = 'failed', attempts = $2, last_error = $3 WHERE id = $1::uuid`, id, attempts, truncate(lastErr))
}

func (s *Store) updateDelivery(ctx context.Context, op, sql string, args ...any) error {
	tag, err := s.db.Exec(ctx, sql, args...)
	if err != nil {
		if isInvalidText(err) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("%s: %w", op, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func truncate(s string) string {
	r := []rune(s)
	if len(r) > maxLastErrorRunes {
		return string(r[:maxLastErrorRunes])
	}
	return s
}

func scanNotification(row pgx.Row) (domain.Notification, error) {
	var n domain.Notification
	return n, scanNotificationInto(row, &n)
}

func scanNotificationInto(row pgx.Row, n *domain.Notification) error {
	var (
		kind    string
		payload []byte
	)
	dest := []any{&n.ID, &n.UserID, &n.PetID, &n.TagID, &n.ScanID, &n.MessageID, &kind, &payload, &n.ReadAt, &n.CreatedAt}
	if err := row.Scan(dest...); err != nil {
		return err
	}
	n.Kind = domain.NotificationKind(kind)
	if err := json.Unmarshal(payload, &n.Payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	n.CreatedAt = n.CreatedAt.UTC()
	if n.ReadAt != nil {
		v := n.ReadAt.UTC()
		n.ReadAt = &v
	}
	return nil
}

func scanClaimed(row pgx.Row, d *domain.Delivery) (domain.Notification, error) {
	var (
		n       domain.Notification
		kind    string
		payload []byte
	)
	if err := row.Scan(&d.ID, &d.NotificationID, &d.Attempts, &n.ID, &n.UserID, &n.PetID, &n.TagID, &n.ScanID,
		&n.MessageID, &kind, &payload, &n.ReadAt, &n.CreatedAt, &d.ToEmail, &d.OwnerName); err != nil {
		return n, err
	}
	n.Kind = domain.NotificationKind(kind)
	if err := json.Unmarshal(payload, &n.Payload); err != nil {
		return n, fmt.Errorf("decode payload: %w", err)
	}
	n.CreatedAt = n.CreatedAt.UTC()
	if n.ReadAt != nil {
		v := n.ReadAt.UTC()
		n.ReadAt = &v
	}
	return n, nil
}

func isInvalidText(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == invalidTextRepresentation
}
