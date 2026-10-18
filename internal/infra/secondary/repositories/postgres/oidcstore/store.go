package oidcstore

import (
	"context"
	"errors"
	"time"

	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const defaultPurgeBatchSize = 500

// Store persists OIDC transactions and external identities.
type Store struct {
	db             *database.PostgresPgx
	purgeBatchSize int
}

func New(db *database.PostgresPgx) *Store {
	return NewWithPurgeBatchSize(db, defaultPurgeBatchSize)
}

func NewWithPurgeBatchSize(db *database.PostgresPgx, purgeBatchSize int) *Store {
	if purgeBatchSize <= 0 {
		purgeBatchSize = defaultPurgeBatchSize
	}
	return &Store{db: db, purgeBatchSize: purgeBatchSize}
}

var (
	_ ports.OIDCTransactionStore  = (*Store)(nil)
	_ ports.ExternalIdentityStore = (*Store)(nil)
)

func (s *Store) Create(ctx context.Context, tx domain.OIDCTransaction) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	actor := nullableUUID(tx.ActorUserID)
	const q = `
INSERT INTO public.oidc_transactions
  (provider, state_hash, nonce, pkce_verifier_ciphertext, intent, actor_user_id, return_to, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := s.db.Exec(ctx, q,
		tx.Provider,
		tx.StateHash,
		tx.Nonce,
		tx.VerifierCipher,
		string(tx.Intent),
		actor,
		tx.ReturnTo,
		tx.ExpiresAt,
	)
	return err
}

func (s *Store) ConsumeByStateHash(ctx context.Context, stateHash []byte, at time.Time) (domain.OIDCTransaction, error) {
	if err := ctx.Err(); err != nil {
		return domain.OIDCTransaction{}, err
	}
	const q = `
UPDATE public.oidc_transactions
SET consumed_at = $2
WHERE state_hash = $1
  AND consumed_at IS NULL
  AND expires_at > $2
RETURNING id, provider, state_hash, nonce, pkce_verifier_ciphertext, intent,
          COALESCE(actor_user_id::text, ''), return_to, expires_at`
	tx, err := scanTransaction(s.db.QueryRow(ctx, q, stateHash, at))
	if err == nil {
		return tx, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.OIDCTransaction{}, err
	}
	var expiresAt time.Time
	var consumed *time.Time
	lookup := s.db.QueryRow(ctx, `
SELECT expires_at, consumed_at
FROM public.oidc_transactions
WHERE state_hash = $1`, stateHash)
	if scanErr := lookup.Scan(&expiresAt, &consumed); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return domain.OIDCTransaction{}, domain.ErrOIDCInvalidState
		}
		return domain.OIDCTransaction{}, scanErr
	}
	if consumed != nil {
		return domain.OIDCTransaction{}, domain.ErrOIDCReplay
	}
	if !expiresAt.After(at) {
		return domain.OIDCTransaction{}, domain.ErrOIDCExpired
	}
	return domain.OIDCTransaction{}, domain.ErrOIDCInvalidState
}

func (s *Store) AttachGrant(ctx context.Context, id string, grantHash []byte, userID string, expiresAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `
UPDATE public.oidc_transactions
SET grant_hash = $2, grant_expires_at = $3, resolved_user_id = $4::uuid
WHERE id = $1::uuid AND grant_hash IS NULL`, id, grantHash, expiresAt, userID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOIDCInvalidState
	}
	return nil
}

func (s *Store) ConsumeGrant(ctx context.Context, grantHash []byte, at time.Time) (domain.OIDCTransaction, error) {
	if err := ctx.Err(); err != nil {
		return domain.OIDCTransaction{}, err
	}
	const q = `
UPDATE public.oidc_transactions
SET grant_consumed_at = $2
WHERE grant_hash = $1
  AND grant_consumed_at IS NULL
  AND grant_expires_at > $2
RETURNING id, provider, state_hash, nonce, pkce_verifier_ciphertext, intent,
          COALESCE(actor_user_id::text, ''), return_to, expires_at, COALESCE(resolved_user_id::text, '')`
	row := s.db.QueryRow(ctx, q, grantHash, at)
	var tx domain.OIDCTransaction
	var intent string
	if err := row.Scan(
		&tx.ID, &tx.Provider, &tx.StateHash, &tx.Nonce, &tx.VerifierCipher, &intent,
		&tx.ActorUserID, &tx.ReturnTo, &tx.ExpiresAt, &tx.ResolvedUserID,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var expiresAt *time.Time
			var consumed *time.Time
			lookup := s.db.QueryRow(ctx, `
SELECT grant_expires_at, grant_consumed_at
FROM public.oidc_transactions
WHERE grant_hash = $1`, grantHash)
			if scanErr := lookup.Scan(&expiresAt, &consumed); scanErr != nil {
				if errors.Is(scanErr, pgx.ErrNoRows) {
					return domain.OIDCTransaction{}, domain.ErrOIDCInvalidGrant
				}
				return domain.OIDCTransaction{}, scanErr
			}
			if consumed != nil {
				return domain.OIDCTransaction{}, domain.ErrOIDCReplay
			}
			if expiresAt == nil || !expiresAt.After(at) {
				return domain.OIDCTransaction{}, domain.ErrOIDCExpired
			}
			return domain.OIDCTransaction{}, domain.ErrOIDCInvalidGrant
		}
		return domain.OIDCTransaction{}, err
	}
	tx.Intent = domain.OIDCIntent(intent)
	return tx, nil
}

func (s *Store) PurgeExpired(ctx context.Context, before time.Time) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	tag, err := s.db.Exec(ctx, `
DELETE FROM public.oidc_transactions
WHERE ctid IN (
    SELECT ctid
    FROM public.oidc_transactions
    WHERE expires_at < $1
      AND (grant_hash IS NULL OR grant_expires_at IS NULL OR grant_expires_at < $1)
    LIMIT $2
)`, before, s.purgeBatchSize)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *Store) FindByIssuerSubject(ctx context.Context, issuer, subject string) (domain.ExternalIdentity, error) {
	if err := ctx.Err(); err != nil {
		return domain.ExternalIdentity{}, err
	}
	const q = `
SELECT id::text, provider, issuer, subject, user_id::text, COALESCE(email_at_link, ''),
       last_login_at, created_at
FROM public.external_identities
WHERE issuer = $1 AND subject = $2`
	var identity domain.ExternalIdentity
	err := s.db.QueryRow(ctx, q, issuer, subject).Scan(
		&identity.ID, &identity.Provider, &identity.Issuer, &identity.Subject,
		&identity.UserID, &identity.EmailAtLink, &identity.LastLoginAt, &identity.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ExternalIdentity{}, domain.ErrNotFound
	}
	return identity, err
}

func (s *Store) Link(ctx context.Context, identity domain.ExternalIdentity) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `
INSERT INTO public.external_identities (provider, issuer, subject, user_id, email_at_link)
VALUES ($1, $2, $3, $4::uuid, $5)`,
		identity.Provider, identity.Issuer, identity.Subject, identity.UserID, nullableString(identity.EmailAtLink),
	)
	if isUniqueViolation(err) {
		return domain.ErrConflict
	}
	return err
}

func (s *Store) MarkLogin(ctx context.Context, id string, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `
UPDATE public.external_identities SET last_login_at = $2 WHERE id = $1::uuid`, id, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) Unlink(ctx context.Context, userID, provider string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `
DELETE FROM public.external_identities WHERE user_id = $1::uuid AND provider = $2`, userID, provider)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanTransaction(row pgx.Row) (domain.OIDCTransaction, error) {
	var tx domain.OIDCTransaction
	var intent string
	err := row.Scan(
		&tx.ID, &tx.Provider, &tx.StateHash, &tx.Nonce, &tx.VerifierCipher, &intent,
		&tx.ActorUserID, &tx.ReturnTo, &tx.ExpiresAt,
	)
	if err != nil {
		return domain.OIDCTransaction{}, err
	}
	tx.Intent = domain.OIDCIntent(intent)
	return tx, nil
}

func nullableUUID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
