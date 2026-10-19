// Package tagstore implements the pet tag port on PostgreSQL.
package tagstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	tagColumns = `id::text, code, kind, status, COALESCE(batch_id::text, ''), COALESCE(pet_id::text, ''),
COALESCE(owner_id::text, ''), activated_at, disabled_at, created_at`
	batchColumns = `id::text, label, quantity, COALESCE(created_by::text, ''), created_at`

	codeUniqueIndex = "pet_tags_code_uq"
	defaultPageSize = 20
)

// Store persists pet tags and batches.
type Store struct {
	db *database.PostgresPgx
}

func New(db *database.PostgresPgx) *Store {
	return &Store{db: db}
}

var _ ports.TagPort = (*Store)(nil)

func (s *Store) CreateBatch(ctx context.Context, batch domain.TagBatch, codes []string) (domain.TagBatch, error) {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return domain.TagBatch{}, fmt.Errorf("begin tag batch: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()

	var createdBy any
	if batch.CreatedBy != "" {
		createdBy = batch.CreatedBy
	}
	row := tx.QueryRow(ctx, `INSERT INTO public.tag_batches (label, quantity, created_by)
VALUES ($1, $2, $3::uuid)
RETURNING `+batchColumns, strings.TrimSpace(batch.Label), len(codes), createdBy)
	created, err := scanBatch(row)
	if err != nil {
		return domain.TagBatch{}, fmt.Errorf("insert tag batch: %w", err)
	}
	for _, code := range codes {
		if _, err := tx.Exec(ctx, `INSERT INTO public.pet_tags (code, kind, status, batch_id)
VALUES ($1, 'physical', 'unassigned', $2::uuid)`, code, created.ID); err != nil {
			if isCodeConflict(err) {
				return domain.TagBatch{}, domain.ErrDuplicateTagCode
			}
			return domain.TagBatch{}, fmt.Errorf("insert tag: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.TagBatch{}, fmt.Errorf("commit tag batch: %w", err)
	}
	committed = true
	return created, nil
}

func (s *Store) ListBatches(ctx context.Context, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.TagBatch], error) {
	limit := page.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	var (
		rows pgx.Rows
		err  error
	)
	if page.After == nil {
		rows, err = s.db.Query(ctx, `SELECT `+batchColumns+`
FROM public.tag_batches
ORDER BY created_at DESC, id DESC
LIMIT $1`, limit+1)
	} else {
		rows, err = s.db.Query(ctx, `SELECT `+batchColumns+`
FROM public.tag_batches
WHERE (created_at, id) < ($1, $2::uuid)
ORDER BY created_at DESC, id DESC
LIMIT $3`, page.After.CreatedAt, page.After.ID, limit+1)
	}
	if err != nil {
		return authBaseDomain.Page[domain.TagBatch]{}, fmt.Errorf("list tag batches: %w", err)
	}
	defer rows.Close()
	items := make([]domain.TagBatch, 0, limit)
	for rows.Next() {
		batch, err := scanBatch(rows)
		if err != nil {
			return authBaseDomain.Page[domain.TagBatch]{}, fmt.Errorf("scan tag batch: %w", err)
		}
		items = append(items, batch)
	}
	if err := rows.Err(); err != nil {
		return authBaseDomain.Page[domain.TagBatch]{}, fmt.Errorf("list tag batches: %w", err)
	}
	hasNext := len(items) > limit
	if hasNext {
		items = items[:limit]
	}
	return authBaseDomain.Page[domain.TagBatch]{Items: items, HasNext: hasNext}, nil
}

func (s *Store) BatchTags(ctx context.Context, batchID string) ([]domain.PetTag, error) {
	return s.queryTags(ctx, "list batch tags", `SELECT `+tagColumns+`
FROM public.pet_tags
WHERE batch_id = $1::uuid
ORDER BY created_at, code`, batchID)
}

func (s *Store) GetByCode(ctx context.Context, code string) (domain.PetTag, error) {
	row := s.db.QueryRow(ctx, `SELECT `+tagColumns+` FROM public.pet_tags WHERE code = $1`, code)
	return singleTag(row, "get tag")
}

func (s *Store) ListByPet(ctx context.Context, ownerID, petID string) ([]domain.PetTag, error) {
	return s.queryTags(ctx, "list pet tags", `SELECT `+tagColumns+`
FROM public.pet_tags
WHERE pet_id = $2::uuid AND owner_id = $1::uuid
ORDER BY created_at DESC, id DESC`, ownerID, petID)
}

func (s *Store) Activate(ctx context.Context, code, ownerID, petID string, maxActive int, at time.Time) (domain.PetTag, error) {
	var tag domain.PetTag
	err := s.withPetLock(ctx, ownerID, petID, maxActive, func(tx database.Transaction) error {
		row := tx.QueryRow(ctx, `UPDATE public.pet_tags
SET status = 'active', pet_id = $3::uuid, owner_id = $2::uuid, activated_at = $4, disabled_at = NULL
WHERE code = $1 AND status = 'unassigned' AND (owner_id IS NULL OR owner_id = $2::uuid)
RETURNING `+tagColumns, code, ownerID, petID, at)
		var err error
		tag, err = singleTag(row, "activate tag")
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrTagNotAssignable
		}
		return err
	})
	if err != nil {
		return domain.PetTag{}, err
	}
	return tag, nil
}

func (s *Store) CreateDigital(ctx context.Context, ownerID, petID, code string, maxActive int, at time.Time) (domain.PetTag, error) {
	var tag domain.PetTag
	err := s.withPetLock(ctx, ownerID, petID, maxActive, func(tx database.Transaction) error {
		row := tx.QueryRow(ctx, `INSERT INTO public.pet_tags (code, kind, status, pet_id, owner_id, activated_at)
VALUES ($1, 'digital', 'active', $2::uuid, $3::uuid, $4)
RETURNING `+tagColumns, code, petID, ownerID, at)
		var err error
		tag, err = scanTag(row)
		if err != nil {
			if isCodeConflict(err) {
				return domain.ErrDuplicateTagCode
			}
			return fmt.Errorf("create digital tag: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.PetTag{}, err
	}
	return tag, nil
}

// withPetLock runs write in a transaction that holds a row lock on the owner's
// pet, so concurrent activations of the same pet serialize and the active-tag
// limit cannot be exceeded.
func (s *Store) withPetLock(ctx context.Context, ownerID, petID string, maxActive int, write func(tx database.Transaction) error) error {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tag write: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	var locked int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM public.pets WHERE id = $1::uuid AND owner_id = $2::uuid FOR UPDATE`, petID, ownerID).Scan(&locked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("lock pet: %w", err)
	}
	var active int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.pet_tags WHERE pet_id = $1::uuid AND status = 'active'`, petID).Scan(&active); err != nil {
		return fmt.Errorf("count active tags: %w", err)
	}
	if active >= maxActive {
		return domain.ErrTagLimitReached
	}
	if err := write(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tag write: %w", err)
	}
	committed = true
	return nil
}

func (s *Store) Disable(ctx context.Context, ownerID, petID, tagID string, at time.Time) (domain.PetTag, error) {
	row := s.db.QueryRow(ctx, `UPDATE public.pet_tags
SET status = 'disabled', disabled_at = $4
WHERE id = $3::uuid AND pet_id = $2::uuid AND owner_id = $1::uuid AND status = 'active'
RETURNING `+tagColumns, ownerID, petID, tagID, at)
	return singleTag(row, "disable tag")
}

func (s *Store) queryTags(ctx context.Context, op, sql string, args ...any) ([]domain.PetTag, error) {
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()
	tags := make([]domain.PetTag, 0)
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return tags, nil
}

func singleTag(row pgx.Row, op string) (domain.PetTag, error) {
	tag, err := scanTag(row)
	if err == nil {
		return tag, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PetTag{}, domain.ErrNotFound
	}
	return domain.PetTag{}, fmt.Errorf("%s: %w", op, err)
}

func scanTag(row pgx.Row) (domain.PetTag, error) {
	var (
		tag          domain.PetTag
		kind, status string
		code         string
	)
	if err := row.Scan(&tag.ID, &code, &kind, &status, &tag.BatchID, &tag.PetID, &tag.OwnerID,
		&tag.ActivatedAt, &tag.DisabledAt, &tag.CreatedAt); err != nil {
		return domain.PetTag{}, err
	}
	tag.Code = strings.TrimSpace(code)
	tag.Kind = domain.TagKind(kind)
	tag.Status = domain.TagStatus(status)
	tag.CreatedAt = tag.CreatedAt.UTC()
	utc(&tag.ActivatedAt)
	utc(&tag.DisabledAt)
	return tag, nil
}

func scanBatch(row pgx.Row) (domain.TagBatch, error) {
	var batch domain.TagBatch
	if err := row.Scan(&batch.ID, &batch.Label, &batch.Quantity, &batch.CreatedBy, &batch.CreatedAt); err != nil {
		return domain.TagBatch{}, err
	}
	batch.CreatedAt = batch.CreatedAt.UTC()
	return batch, nil
}

func utc(t **time.Time) {
	if *t != nil {
		v := (*t).UTC()
		*t = &v
	}
}

func isCodeConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == codeUniqueIndex
}
