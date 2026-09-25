package refreshstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/WilsonSayago/authBase/v4/core"
	"github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/authBase/v4/core/port"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const defaultPurgeBatchSize = 500

// Store persists refresh sessions in PostgreSQL with atomic Rotate.
type Store struct {
	db             *database.PostgresPgx
	now            func() time.Time
	purgeBatchSize int
}

// New returns a RefreshTokenStore backed by Postgres.
func New(db *database.PostgresPgx) *Store {
	return NewWithPurgeBatchSize(db, defaultPurgeBatchSize)
}

// NewWithPurgeBatchSize constructs a store with bounded expiry cleanup.
func NewWithPurgeBatchSize(db *database.PostgresPgx, purgeBatchSize int) *Store {
	if purgeBatchSize <= 0 {
		purgeBatchSize = defaultPurgeBatchSize
	}
	return &Store{db: db, now: time.Now, purgeBatchSize: purgeBatchSize}
}

var (
	_ port.RefreshTokenStore    = (*Store)(nil)
	_ port.UserSessionRevoker   = (*Store)(nil)
	_ port.ExpiredSessionPurger = (*Store)(nil)
)

func (s *Store) Create(ctx context.Context, session domain.RefreshSession) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !session.Valid() {
		return fmt.Errorf("invalid refresh session")
	}
	const q = `
INSERT INTO public.refresh_sessions
  (token_hash, token_id, family_id, user_id, issued_at, expires_at)
VALUES ($1, $2, $3, $4::uuid, $5, $6)`
	_, err := s.db.Exec(ctx, q,
		session.TokenHash[:],
		session.TokenID,
		session.FamilyID,
		session.UserID,
		session.IssuedAt.UTC(),
		session.ExpiresAt.UTC(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("refresh session already exists: %w", err)
		}
		return fmt.Errorf("create refresh session: %w", err)
	}
	return nil
}

func (s *Store) Rotate(ctx context.Context, currentHash [32]byte, next domain.RefreshSession) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !next.Valid() {
		return fmt.Errorf("invalid refresh session")
	}

	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin rotate tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		familyID              string
		expiresAt             time.Time
		consumedAt, revokedAt *time.Time
	)
	err = tx.QueryRow(ctx, `
SELECT family_id, expires_at, consumed_at, revoked_at
FROM public.refresh_sessions
WHERE token_hash = $1
FOR UPDATE`, currentHash[:]).Scan(&familyID, &expiresAt, &consumedAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ErrRefreshNotFound
		}
		return fmt.Errorf("load refresh session: %w", err)
	}

	var familyRevoked bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM public.refresh_sessions
  WHERE family_id = $1 AND revoked_at IS NOT NULL
)`, familyID).Scan(&familyRevoked); err != nil {
		return fmt.Errorf("check family revoke: %w", err)
	}
	if familyRevoked || revokedAt != nil {
		return core.ErrRefreshFamilyRevoked
	}
	if consumedAt != nil {
		return core.ErrRefreshConsumed
	}
	now := s.now().UTC()
	if !now.Before(expiresAt.UTC()) {
		return core.ErrRefreshNotFound
	}
	if next.FamilyID != familyID {
		return fmt.Errorf("refresh family mismatch")
	}

	tag, err := tx.Exec(ctx, `
UPDATE public.refresh_sessions
SET consumed_at = $2
WHERE token_hash = $1 AND consumed_at IS NULL AND revoked_at IS NULL`,
		currentHash[:], now)
	if err != nil {
		return fmt.Errorf("consume refresh session: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return core.ErrRefreshConsumed
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO public.refresh_sessions
  (token_hash, token_id, family_id, user_id, issued_at, expires_at)
VALUES ($1, $2, $3, $4::uuid, $5, $6)`,
		next.TokenHash[:],
		next.TokenID,
		next.FamilyID,
		next.UserID,
		next.IssuedAt.UTC(),
		next.ExpiresAt.UTC(),
	); err != nil {
		if isUniqueViolation(err) {
			return core.ErrRefreshConsumed
		}
		return fmt.Errorf("insert next refresh session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit rotate: %w", err)
	}
	return nil
}

func (s *Store) RevokeFamily(ctx context.Context, familyID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if familyID == "" {
		return fmt.Errorf("family id must not be empty")
	}
	now := s.now().UTC()
	if _, err := s.db.Exec(ctx, `
UPDATE public.refresh_sessions
SET revoked_at = $2
WHERE family_id = $1 AND revoked_at IS NULL`, familyID, now); err != nil {
		return fmt.Errorf("revoke refresh family: %w", err)
	}
	return nil
}

// RevokeUser invalidates every non-revoked refresh session owned by userID.
func (s *Store) RevokeUser(ctx context.Context, userID string) error {
	return s.revokeUser(ctx, s.db, userID)
}

// RevokeUserTx performs user-wide revocation in an existing transaction.
func (s *Store) RevokeUserTx(ctx context.Context, tx database.Transaction, userID string) error {
	if tx == nil {
		return fmt.Errorf("revoke user transaction is nil")
	}
	return s.revokeUser(ctx, tx, userID)
}

func (s *Store) revokeUser(ctx context.Context, db database.DBTX, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if userID == "" {
		return fmt.Errorf("user id must not be empty")
	}
	if _, err := db.Exec(ctx, `
UPDATE public.refresh_sessions
SET revoked_at = $2
WHERE user_id = $1::uuid AND revoked_at IS NULL`, userID, s.now().UTC()); err != nil {
		return fmt.Errorf("revoke user refresh sessions: %w", err)
	}
	return nil
}

// PurgeExpired deletes at most the configured batch size of sessions whose
// expiry is strictly before the caller-selected retention cutoff.
func (s *Store) PurgeExpired(ctx context.Context, before time.Time) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if before.IsZero() {
		return 0, fmt.Errorf("purge cutoff must not be zero")
	}
	tag, err := s.db.Exec(ctx, `
WITH expired AS (
	SELECT token_hash
	FROM public.refresh_sessions
	WHERE expires_at < $1
	ORDER BY expires_at
	LIMIT $2
	FOR UPDATE SKIP LOCKED
)
DELETE FROM public.refresh_sessions AS sessions
USING expired
WHERE sessions.token_hash = expired.token_hash`, before.UTC(), s.purgeBatchSize)
	if err != nil {
		return 0, fmt.Errorf("purge expired refresh sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
