package notifystore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/jackc/pgx/v5"
)

// InsertEvent records a notification (and, when draft.Email, its email
// delivery) inside the caller's transaction. A tag_scanned event schedules its
// email FirstEmailDelay later. Location, message and contact events of the same
// scan merge into that pending scan email instead of sending their own, as long
// as it has not been claimed or attempted yet; the in-app notice is always
// created. scanID and messageID may be empty (stored as NULL).
func InsertEvent(ctx context.Context, tx database.DBTX, draft domain.NotificationDraft, scanID, messageID string, now time.Time) error {
	payload, err := json.Marshal(draft.Payload)
	if err != nil {
		return fmt.Errorf("encode notification payload: %w", err)
	}
	var notificationID string
	if err := tx.QueryRow(ctx, `INSERT INTO public.notifications (user_id, kind, pet_id, tag_id, scan_id, message_id, payload, created_at)
VALUES ($1::uuid, $2, $3::uuid, $4::uuid, $5::uuid, $6::uuid, $7::jsonb, $8)
RETURNING id::text`, draft.UserID, string(draft.Kind), draft.PetID, draft.TagID, nullable(scanID), nullable(messageID), payload, now).Scan(&notificationID); err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	if !draft.Email {
		return nil
	}
	nextAttempt := now
	switch draft.Kind {
	case domain.NotificationTagScanned:
		nextAttempt = now.Add(domain.FirstEmailDelay)
	default:
		grouped, err := groupIntoScanEmail(ctx, tx, draft, scanID, now)
		if err != nil {
			return err
		}
		if grouped {
			return nil
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.notification_deliveries (notification_id, channel, status, attempts, next_attempt_at)
VALUES ($1::uuid, 'email', 'pending', 0, $2)`, notificationID, nextAttempt); err != nil {
		return fmt.Errorf("insert delivery: %w", err)
	}
	return nil
}

// groupIntoScanEmail merges the event into the payload of the scan's pending,
// never-attempted tag_scanned email. A delivery whose next_attempt_at is past
// created_at + FirstEmailDelay has been leased by a dispatcher, so it is left
// alone and the event sends on its own.
func groupIntoScanEmail(ctx context.Context, tx database.DBTX, draft domain.NotificationDraft, scanID string, now time.Time) (bool, error) {
	if scanID == "" {
		return false, nil
	}
	patch := map[string]any{}
	switch draft.Kind {
	case domain.NotificationLocationShared:
		if draft.Payload.Location == nil {
			return false, nil
		}
		patch["location"] = draft.Payload.Location
	case domain.NotificationFinderMessage:
		if draft.Payload.Message == nil {
			return false, nil
		}
		patch["message"] = draft.Payload.Message
	case domain.NotificationPhoneRevealed:
		patch["phoneRevealed"] = true
	default:
		return false, nil
	}
	var scanNotificationID string
	err := tx.QueryRow(ctx, `SELECT n.id::text
FROM public.notifications n
JOIN public.notification_deliveries d ON d.notification_id = n.id
WHERE n.scan_id = $1::uuid AND n.kind = 'tag_scanned'
  AND d.channel = 'email' AND d.status = 'pending' AND d.attempts = 0
  AND d.next_attempt_at > $2
  AND d.next_attempt_at <= n.created_at + make_interval(secs => $3)
ORDER BY n.created_at DESC
LIMIT 1
FOR UPDATE OF n, d`, scanID, now, domain.FirstEmailDelay.Seconds()).Scan(&scanNotificationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("find pending scan email: %w", err)
	}
	merge, err := json.Marshal(patch)
	if err != nil {
		return false, fmt.Errorf("encode payload patch: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE public.notifications SET payload = payload || $2::jsonb WHERE id = $1::uuid`, scanNotificationID, merge); err != nil {
		return false, fmt.Errorf("merge scan payload: %w", err)
	}
	return true, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
