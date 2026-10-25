package tagstore

import (
	"context"
	"fmt"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

const (
	purgeBatchSize  = 1000
	purgeMaxBatches = 50
)

// PurgeTagData implements ports.TagScanPort. Every statement runs on its own
// (an implicit short transaction), so no long transaction holds locks.
func (s *Store) PurgeTagData(ctx context.Context, now time.Time) (int, int, int, error) {
	if now.IsZero() {
		return 0, 0, 0, fmt.Errorf("purge reference time must not be zero")
	}
	now = now.UTC()
	locationCutoff := now.Add(-domain.LocationRetention)
	scanCutoff := now.Add(-domain.ScanRetention)

	cleared, err := s.db.Exec(ctx, `UPDATE public.tag_scans
SET latitude = NULL, longitude = NULL, accuracy_m = NULL, location_at = NULL
WHERE location_at < $1`, locationCutoff)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("clear scan locations: %w", err)
	}
	if _, err := s.db.Exec(ctx, `UPDATE public.notifications
SET payload = payload - 'location'
WHERE created_at < $1 AND payload ? 'location'`, locationCutoff); err != nil {
		return int(cleared.RowsAffected()), 0, 0, fmt.Errorf("strip notification locations: %w", err)
	}

	notifs, err := s.deleteBatched(ctx, `DELETE FROM public.notifications
WHERE id IN (SELECT id FROM public.notifications WHERE created_at < $1 LIMIT $2)`, scanCutoff)
	if err != nil {
		return int(cleared.RowsAffected()), 0, notifs, fmt.Errorf("delete notifications: %w", err)
	}
	scans, err := s.deleteBatched(ctx, `DELETE FROM public.tag_scans
WHERE id IN (SELECT id FROM public.tag_scans WHERE scanned_at < $1 LIMIT $2)`, scanCutoff)
	if err != nil {
		return int(cleared.RowsAffected()), scans, notifs, fmt.Errorf("delete tag scans: %w", err)
	}
	return int(cleared.RowsAffected()), scans, notifs, nil
}

// deleteBatched runs query ($1 cutoff, $2 batch size) until a batch comes back
// short or purgeMaxBatches is reached, and returns the rows deleted.
func (s *Store) deleteBatched(ctx context.Context, query string, cutoff time.Time) (int, error) {
	total := 0
	for i := 0; i < purgeMaxBatches; i++ {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		tag, err := s.db.Exec(ctx, query, cutoff, purgeBatchSize)
		if err != nil {
			return total, err
		}
		n := int(tag.RowsAffected())
		total += n
		if n < purgeBatchSize {
			break
		}
	}
	return total, nil
}
