package tagstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/notifystore"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	scanColumns = `id::text, tag_id::text, pet_id::text, scanned_at, visitor_hash, scan_token_hash,
ip_prefix, user_agent_family, phone_revealed_at, latitude::float8, longitude::float8, accuracy_m, location_at`
	messageColumns = `id::text, scan_id::text, tag_id::text, pet_id::text, body, finder_name, finder_phone, created_at`

	invalidTextRepresentation = "22P02"
)

var _ ports.TagScanPort = (*Store)(nil)

// RecordScan dedupes by (tag, visitor) inside one transaction. A transaction
// advisory lock on that pair serializes concurrent first scans, which a row
// lock alone cannot do because there is no row to lock yet.
func (s *Store) RecordScan(ctx context.Context, scan domain.TagScan, dedupeSince time.Time, draft *domain.NotificationDraft) (domain.TagScan, bool, error) {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return domain.TagScan{}, false, fmt.Errorf("begin record scan: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text || $2::text))`, scan.TagID, scan.VisitorHash); err != nil {
		return domain.TagScan{}, false, fmt.Errorf("lock scan visitor: %w", err)
	}

	var existingID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM public.tag_scans
WHERE tag_id = $1::uuid AND visitor_hash = $2 AND scanned_at >= $3
ORDER BY scanned_at DESC, id DESC
LIMIT 1
FOR UPDATE`, scan.TagID, scan.VisitorHash, dedupeSince).Scan(&existingID)

	var (
		stored domain.TagScan
		reused bool
	)
	switch {
	case err == nil:
		stored, err = scanScan(tx.QueryRow(ctx, `UPDATE public.tag_scans SET scan_token_hash = $2
WHERE id = $1::uuid
RETURNING `+scanColumns, existingID, scan.ScanTokenHash))
		if err != nil {
			return domain.TagScan{}, false, fmt.Errorf("refresh scan token: %w", err)
		}
		reused = true
	case errors.Is(err, pgx.ErrNoRows):
		var scannedAt any
		if !scan.ScannedAt.IsZero() {
			scannedAt = scan.ScannedAt
		}
		stored, err = scanScan(tx.QueryRow(ctx, `INSERT INTO public.tag_scans
(tag_id, pet_id, scanned_at, visitor_hash, scan_token_hash, ip_prefix, user_agent_family)
VALUES ($1::uuid, $2::uuid, COALESCE($3::timestamptz, now()), $4, $5, $6, $7)
RETURNING `+scanColumns, scan.TagID, scan.PetID, scannedAt, scan.VisitorHash, scan.ScanTokenHash,
			scan.IPPrefix, scan.UserAgentFamily))
		if err != nil {
			return domain.TagScan{}, false, fmt.Errorf("insert scan: %w", err)
		}
		if draft != nil {
			if err := notifystore.InsertEvent(ctx, tx, *draft, stored.ID, "", stored.ScannedAt); err != nil {
				return domain.TagScan{}, false, fmt.Errorf("notify scan: %w", err)
			}
		}
	default:
		return domain.TagScan{}, false, fmt.Errorf("find recent scan: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.TagScan{}, false, fmt.Errorf("commit record scan: %w", err)
	}
	committed = true
	return stored, reused, nil
}

func (s *Store) GetScan(ctx context.Context, scanID string) (domain.TagScan, error) {
	scan, err := scanScan(s.db.QueryRow(ctx, `SELECT `+scanColumns+` FROM public.tag_scans WHERE id = $1::uuid`, scanID))
	if err != nil {
		if isNoRowsOrBadUUID(err) {
			return domain.TagScan{}, domain.ErrNotFound
		}
		return domain.TagScan{}, fmt.Errorf("get scan: %w", err)
	}
	return scan, nil
}

func (s *Store) AddMessage(ctx context.Context, msg domain.TagMessage, draft *domain.NotificationDraft) (domain.TagMessage, error) {
	var createdAt any
	if !msg.CreatedAt.IsZero() {
		createdAt = msg.CreatedAt
	}
	var stored domain.TagMessage
	err := s.inTx(ctx, "add message", func(tx database.Transaction) error {
		var err error
		stored, err = scanMessage(tx.QueryRow(ctx, `INSERT INTO public.tag_messages
(scan_id, tag_id, pet_id, body, finder_name, finder_phone, created_at)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, COALESCE($7::timestamptz, now()))
RETURNING `+messageColumns, msg.ScanID, msg.TagID, msg.PetID, msg.Body, msg.FinderName, msg.FinderPhone, createdAt))
		if err != nil {
			return fmt.Errorf("insert tag message: %w", err)
		}
		if draft != nil {
			if err := notifystore.InsertEvent(ctx, tx, *draft, stored.ScanID, stored.ID, stored.CreatedAt); err != nil {
				return fmt.Errorf("notify message: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return domain.TagMessage{}, err
	}
	return stored, nil
}

func (s *Store) MarkPhoneRevealed(ctx context.Context, scanID string, at time.Time, draft *domain.NotificationDraft) (bool, error) {
	revealed := false
	err := s.inTx(ctx, "mark phone revealed", func(tx database.Transaction) error {
		tag, err := tx.Exec(ctx, `UPDATE public.tag_scans SET phone_revealed_at = $2
WHERE id = $1::uuid AND phone_revealed_at IS NULL`, scanID, at)
		if err != nil {
			if isNoRowsOrBadUUID(err) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("mark phone revealed: %w", err)
		}
		if tag.RowsAffected() == 1 {
			revealed = true
			if draft != nil {
				if err := notifystore.InsertEvent(ctx, tx, *draft, scanID, "", at); err != nil {
					return fmt.Errorf("notify phone revealed: %w", err)
				}
			}
			return nil
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.tag_scans WHERE id = $1::uuid)`, scanID).Scan(&exists); err != nil {
			return fmt.Errorf("check scan: %w", err)
		}
		if !exists {
			return domain.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return revealed, nil
}

func (s *Store) SetLocation(ctx context.Context, scanID string, loc domain.ScanLocation, draft *domain.NotificationDraft) error {
	return s.inTx(ctx, "set location", func(tx database.Transaction) error {
		res, err := tx.Exec(ctx, `UPDATE public.tag_scans
SET latitude = $2, longitude = $3, accuracy_m = $4, location_at = $5
WHERE id = $1::uuid AND location_at IS NULL`, scanID, loc.Lat, loc.Lng, loc.AccuracyM, loc.At)
		if err != nil {
			if isNoRowsOrBadUUID(err) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("set location: %w", err)
		}
		if res.RowsAffected() == 1 {
			if draft != nil {
				if err := notifystore.InsertEvent(ctx, tx, *draft, scanID, "", loc.At); err != nil {
					return fmt.Errorf("notify location: %w", err)
				}
			}
			return nil
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.tag_scans WHERE id = $1::uuid)`, scanID).Scan(&exists); err != nil {
			return fmt.Errorf("check scan: %w", err)
		}
		if !exists {
			return domain.ErrNotFound
		}
		return domain.ErrLocationAlreadyShared
	})
}

// inTx runs fn in a transaction, committing on success and rolling back on any error.
func (s *Store) inTx(ctx context.Context, what string, fn func(tx database.Transaction) error) error {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin %s: %w", what, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", what, err)
	}
	committed = true
	return nil
}

func (s *Store) ListScansByPet(ctx context.Context, ownerID, petID string, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.ScanWithMessages], error) {
	empty := authBaseDomain.Page[domain.ScanWithMessages]{Items: []domain.ScanWithMessages{}}
	limit := page.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	const selectScans = `SELECT s.id::text, s.tag_id::text, s.pet_id::text, s.scanned_at, s.visitor_hash, s.scan_token_hash,
s.ip_prefix, s.user_agent_family, s.phone_revealed_at, s.latitude::float8, s.longitude::float8, s.accuracy_m, s.location_at, t.code
FROM public.tag_scans s
JOIN public.pets p ON p.id = s.pet_id
JOIN public.pet_tags t ON t.id = s.tag_id
WHERE s.pet_id = $1::uuid AND p.owner_id = $2::uuid`
	var (
		rows pgx.Rows
		err  error
	)
	if page.After == nil {
		rows, err = s.db.Query(ctx, selectScans+`
ORDER BY s.scanned_at DESC, s.id DESC
LIMIT $3`, petID, ownerID, limit+1)
	} else {
		rows, err = s.db.Query(ctx, selectScans+`
AND (s.scanned_at, s.id) < ($3, $4::uuid)
ORDER BY s.scanned_at DESC, s.id DESC
LIMIT $5`, petID, ownerID, page.After.CreatedAt, page.After.ID, limit+1)
	}
	if err != nil {
		if isNoRowsOrBadUUID(err) {
			return empty, nil
		}
		return empty, fmt.Errorf("list scans: %w", err)
	}
	defer rows.Close()
	items := make([]domain.ScanWithMessages, 0, limit)
	for rows.Next() {
		var (
			item       domain.ScanWithMessages
			code       string
			lat, lng   *float64
			accuracy   *int
			locationAt *time.Time
		)
		if err := rows.Scan(&item.ID, &item.TagID, &item.PetID, &item.ScannedAt, &item.VisitorHash, &item.ScanTokenHash,
			&item.IPPrefix, &item.UserAgentFamily, &item.PhoneRevealedAt, &lat, &lng, &accuracy, &locationAt, &code); err != nil {
			return empty, fmt.Errorf("scan scan row: %w", err)
		}
		item.ScannedAt = item.ScannedAt.UTC()
		utc(&item.PhoneRevealedAt)
		item.Location = buildLocation(lat, lng, accuracy, locationAt)
		item.TagCode = trimCode(code)
		item.Messages = []domain.TagMessage{}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		if isNoRowsOrBadUUID(err) {
			return empty, nil
		}
		return empty, fmt.Errorf("list scans: %w", err)
	}
	rows.Close()

	hasNext := len(items) > limit
	if hasNext {
		items = items[:limit]
	}
	if len(items) == 0 {
		return empty, nil
	}

	ids := make([]string, len(items))
	index := make(map[string]int, len(items))
	for i, item := range items {
		ids[i] = item.ID
		index[item.ID] = i
	}
	msgRows, err := s.db.Query(ctx, `SELECT `+messageColumns+`
FROM public.tag_messages
WHERE scan_id = ANY($1::uuid[])
ORDER BY created_at, id`, ids)
	if err != nil {
		return empty, fmt.Errorf("list scan messages: %w", err)
	}
	defer msgRows.Close()
	for msgRows.Next() {
		msg, err := scanMessage(msgRows)
		if err != nil {
			return empty, fmt.Errorf("scan message row: %w", err)
		}
		i := index[msg.ScanID]
		items[i].Messages = append(items[i].Messages, msg)
	}
	if err := msgRows.Err(); err != nil {
		return empty, fmt.Errorf("list scan messages: %w", err)
	}
	return authBaseDomain.Page[domain.ScanWithMessages]{Items: items, HasNext: hasNext}, nil
}

func scanScan(row pgx.Row) (domain.TagScan, error) {
	var (
		scan       domain.TagScan
		lat, lng   *float64
		accuracy   *int
		locationAt *time.Time
	)
	if err := row.Scan(&scan.ID, &scan.TagID, &scan.PetID, &scan.ScannedAt, &scan.VisitorHash, &scan.ScanTokenHash,
		&scan.IPPrefix, &scan.UserAgentFamily, &scan.PhoneRevealedAt, &lat, &lng, &accuracy, &locationAt); err != nil {
		return domain.TagScan{}, err
	}
	scan.ScannedAt = scan.ScannedAt.UTC()
	utc(&scan.PhoneRevealedAt)
	scan.Location = buildLocation(lat, lng, accuracy, locationAt)
	return scan, nil
}

// buildLocation returns nil unless every location column is set (the
// retention job nulls them together).
func buildLocation(lat, lng *float64, accuracy *int, at *time.Time) *domain.ScanLocation {
	if lat == nil || lng == nil || accuracy == nil || at == nil {
		return nil
	}
	return &domain.ScanLocation{Lat: *lat, Lng: *lng, AccuracyM: *accuracy, At: at.UTC()}
}

func scanMessage(row pgx.Row) (domain.TagMessage, error) {
	var msg domain.TagMessage
	if err := row.Scan(&msg.ID, &msg.ScanID, &msg.TagID, &msg.PetID, &msg.Body, &msg.FinderName, &msg.FinderPhone, &msg.CreatedAt); err != nil {
		return domain.TagMessage{}, err
	}
	msg.CreatedAt = msg.CreatedAt.UTC()
	return msg, nil
}

func trimCode(code string) string {
	for len(code) > 0 && code[len(code)-1] == ' ' {
		code = code[:len(code)-1]
	}
	return code
}

// isNoRowsOrBadUUID reports a missing row or a malformed uuid argument; both
// mean the identifier does not exist for the caller.
func isNoRowsOrBadUUID(err error) bool {
	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == invalidTextRepresentation
}
