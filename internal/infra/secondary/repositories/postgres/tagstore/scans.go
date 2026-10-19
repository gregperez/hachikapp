package tagstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	scanColumns = `id::text, tag_id::text, pet_id::text, scanned_at, visitor_hash, scan_token_hash,
ip_prefix, user_agent_family, phone_revealed_at`
	messageColumns = `id::text, scan_id::text, tag_id::text, pet_id::text, body, finder_name, finder_phone, created_at`

	invalidTextRepresentation = "22P02"
)

var _ ports.TagScanPort = (*Store)(nil)

// RecordScan dedupes by (tag, visitor) inside one transaction. A transaction
// advisory lock on that pair serializes concurrent first scans, which a row
// lock alone cannot do because there is no row to lock yet.
func (s *Store) RecordScan(ctx context.Context, scan domain.TagScan, dedupeSince time.Time) (domain.TagScan, bool, error) {
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

func (s *Store) AddMessage(ctx context.Context, msg domain.TagMessage) (domain.TagMessage, error) {
	stored, err := scanMessage(s.db.QueryRow(ctx, `INSERT INTO public.tag_messages
(scan_id, tag_id, pet_id, body, finder_name, finder_phone)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6)
RETURNING `+messageColumns, msg.ScanID, msg.TagID, msg.PetID, msg.Body, msg.FinderName, msg.FinderPhone))
	if err != nil {
		return domain.TagMessage{}, fmt.Errorf("insert tag message: %w", err)
	}
	return stored, nil
}

func (s *Store) MarkPhoneRevealed(ctx context.Context, scanID string, at time.Time) (bool, error) {
	tag, err := s.db.Exec(ctx, `UPDATE public.tag_scans SET phone_revealed_at = $2
WHERE id = $1::uuid AND phone_revealed_at IS NULL`, scanID, at)
	if err != nil {
		if isNoRowsOrBadUUID(err) {
			return false, domain.ErrNotFound
		}
		return false, fmt.Errorf("mark phone revealed: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return true, nil
	}
	var exists bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.tag_scans WHERE id = $1::uuid)`, scanID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check scan: %w", err)
	}
	if !exists {
		return false, domain.ErrNotFound
	}
	return false, nil
}

func (s *Store) ListScansByPet(ctx context.Context, ownerID, petID string, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.ScanWithMessages], error) {
	empty := authBaseDomain.Page[domain.ScanWithMessages]{Items: []domain.ScanWithMessages{}}
	limit := page.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	const selectScans = `SELECT s.id::text, s.tag_id::text, s.pet_id::text, s.scanned_at, s.visitor_hash, s.scan_token_hash,
s.ip_prefix, s.user_agent_family, s.phone_revealed_at, t.code
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
			item domain.ScanWithMessages
			code string
		)
		if err := rows.Scan(&item.ID, &item.TagID, &item.PetID, &item.ScannedAt, &item.VisitorHash, &item.ScanTokenHash,
			&item.IPPrefix, &item.UserAgentFamily, &item.PhoneRevealedAt, &code); err != nil {
			return empty, fmt.Errorf("scan scan row: %w", err)
		}
		item.ScannedAt = item.ScannedAt.UTC()
		utc(&item.PhoneRevealedAt)
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
	var scan domain.TagScan
	if err := row.Scan(&scan.ID, &scan.TagID, &scan.PetID, &scan.ScannedAt, &scan.VisitorHash, &scan.ScanTokenHash,
		&scan.IPPrefix, &scan.UserAgentFamily, &scan.PhoneRevealedAt); err != nil {
		return domain.TagScan{}, err
	}
	scan.ScannedAt = scan.ScannedAt.UTC()
	utc(&scan.PhoneRevealedAt)
	return scan, nil
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
