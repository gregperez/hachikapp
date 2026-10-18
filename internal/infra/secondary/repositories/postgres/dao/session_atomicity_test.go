package dao_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	authDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/dao"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/refreshstore"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errForcedRevocation = errors.New("forced revocation failure")

type failingSessionRevoker struct{}

func (failingSessionRevoker) RevokeUserTx(context.Context, database.Transaction, string) error {
	return errForcedRevocation
}

func seedSessionUser(
	t *testing.T,
	pool *pgxpool.Pool,
	ctx context.Context,
	isAdmin bool,
) string {
	t.Helper()
	email := fmt.Sprintf("session-user-%d@example.com", time.Now().UnixNano())
	var id string
	if err := pool.QueryRow(ctx, `
INSERT INTO public.users (name, email, password, is_admin, active)
VALUES ('Session User', $1, 'old-hash', $2, true)
RETURNING id`, email, isAdmin).Scan(&id); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.refresh_sessions WHERE user_id = $1`, id)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.users WHERE id = $1`, id)
	})
	return id
}

func createSession(
	t *testing.T,
	store *refreshstore.Store,
	userID, familyID, tokenID string,
) {
	t.Helper()
	now := time.Now().UTC()
	if err := store.Create(context.Background(), authDomain.RefreshSession{
		TokenID:   tokenID,
		FamilyID:  familyID,
		UserID:    userID,
		TokenHash: sha256.Sum256([]byte(tokenID)),
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatalf("create refresh session: %v", err)
	}
}

func TestUpdatePasswordAndRevokeSessionsIsAtomic(t *testing.T) {
	pool, pg, ctx := openAdminTestDB(t)
	userID := seedSessionUser(t, pool, ctx, false)
	store := refreshstore.New(pg)
	createSession(t, store, userID, "family-a", "token-a")
	createSession(t, store, userID, "family-b", "token-b")

	userDao := dao.NewUserDaoForTest(pg)
	repo := repositories.NewUserRepositoryForTest(userDao, nil, nil, nil, store)
	if err := repo.UpdatePasswordAndRevokeSessions(ctx, userID, "new-hash"); err != nil {
		t.Fatalf("UpdatePasswordAndRevokeSessions() error = %v", err)
	}

	var password string
	var revoked int
	if err := pool.QueryRow(ctx, `SELECT password FROM public.users WHERE id = $1`, userID).Scan(&password); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM public.refresh_sessions WHERE user_id = $1 AND revoked_at IS NOT NULL`,
		userID,
	).Scan(&revoked); err != nil {
		t.Fatal(err)
	}
	if password != "new-hash" || revoked != 2 {
		t.Fatalf("password=%q revoked=%d, want new-hash/2", password, revoked)
	}
}

func TestPasswordAndDeactivationRollbackWhenRevocationFails(t *testing.T) {
	pool, pg, ctx := openAdminTestDB(t)
	userID := seedSessionUser(t, pool, ctx, false)
	repo := repositories.NewUserRepositoryForTest(
		dao.NewUserDaoForTest(pg),
		nil,
		nil,
		nil,
		failingSessionRevoker{},
	)

	err := repo.UpdatePasswordAndRevokeSessions(ctx, userID, "new-hash")
	if !errors.Is(err, errForcedRevocation) {
		t.Fatalf("password update error = %v, want forced revocation", err)
	}
	var password string
	if err := pool.QueryRow(ctx, `SELECT password FROM public.users WHERE id = $1`, userID).Scan(&password); err != nil {
		t.Fatal(err)
	}
	if password != "old-hash" {
		t.Fatalf("password after rollback = %q, want old-hash", password)
	}

	err = repo.DeactivateAndRevokeSessions(ctx, userID)
	if !errors.Is(err, errForcedRevocation) {
		t.Fatalf("deactivation error = %v, want forced revocation", err)
	}
	var active bool
	if err := pool.QueryRow(ctx, `SELECT active FROM public.users WHERE id = $1`, userID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if !active {
		t.Fatal("user remained inactive after revocation rollback")
	}
}

func TestDeactivateAndRevokeSessionsIsolatesOtherUsers(t *testing.T) {
	pool, pg, ctx := openAdminTestDB(t)
	targetID := seedSessionUser(t, pool, ctx, false)
	otherID := seedSessionUser(t, pool, ctx, false)
	store := refreshstore.New(pg)
	createSession(t, store, targetID, "target-family", "target-token")
	createSession(t, store, otherID, "other-family", "other-token")
	repo := repositories.NewUserRepositoryForTest(
		dao.NewUserDaoForTest(pg),
		nil,
		nil,
		nil,
		store,
	)

	if err := repo.DeactivateAndRevokeSessions(ctx, targetID); err != nil {
		t.Fatalf("DeactivateAndRevokeSessions() error = %v", err)
	}

	var targetActive bool
	var targetRevoked, otherRevoked int
	if err := pool.QueryRow(ctx, `SELECT active FROM public.users WHERE id = $1`, targetID).Scan(&targetActive); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM public.refresh_sessions WHERE user_id = $1 AND revoked_at IS NOT NULL`,
		targetID,
	).Scan(&targetRevoked); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM public.refresh_sessions WHERE user_id = $1 AND revoked_at IS NOT NULL`,
		otherID,
	).Scan(&otherRevoked); err != nil {
		t.Fatal(err)
	}
	if targetActive || targetRevoked != 1 || otherRevoked != 0 {
		t.Fatalf(
			"target active=%v revoked=%d other revoked=%d, want false/1/0",
			targetActive,
			targetRevoked,
			otherRevoked,
		)
	}
}

func TestDeactivateAdminAndRevokeSessionsSharesTransaction(t *testing.T) {
	pool, pg, ctx := openAdminTestDB(t)
	targetID := seedSessionUser(t, pool, ctx, true)
	_ = seedSessionUser(t, pool, ctx, true)
	store := refreshstore.New(pg)
	createSession(t, store, targetID, "admin-family", "admin-token")
	repo := repositories.NewUserRepositoryForTest(
		dao.NewUserDaoForTest(pg),
		nil,
		nil,
		nil,
		store,
	)

	if err := repo.DeactivateAdminAndRevokeSessions(ctx, targetID); err != nil {
		t.Fatalf("DeactivateAdminAndRevokeSessions() error = %v", err)
	}

	var active bool
	var revoked int
	if err := pool.QueryRow(ctx, `SELECT active FROM public.users WHERE id = $1`, targetID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM public.refresh_sessions WHERE user_id = $1 AND revoked_at IS NOT NULL`,
		targetID,
	).Scan(&revoked); err != nil {
		t.Fatal(err)
	}
	if active || revoked != 1 {
		t.Fatalf("admin active=%v revoked=%d, want false/1", active, revoked)
	}
}
