package refreshstore_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/WilsonSayago/authBase/v4/core"
	"github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/refreshstore"
	"github.com/WilsonSayago/base-golang/internal/test/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn, err := testdb.RequireURL()
	if err != nil {
		t.Skip(err.Error())
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return pool
}

func setupSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,
		`CREATE TABLE IF NOT EXISTS public.users (
			id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
			name varchar NOT NULL,
			email varchar NOT NULL UNIQUE,
			password varchar NOT NULL,
			is_admin boolean DEFAULT false,
			active boolean DEFAULT true
		)`,
		`CREATE TABLE IF NOT EXISTS public.refresh_sessions (
			token_hash bytea NOT NULL UNIQUE,
			token_id text NOT NULL UNIQUE,
			family_id text NOT NULL,
			user_id uuid NOT NULL REFERENCES public.users(id),
			issued_at timestamptz NOT NULL,
			expires_at timestamptz NOT NULL,
			consumed_at timestamptz NULL,
			revoked_at timestamptz NULL,
			client_label varchar(120) NOT NULL DEFAULT '',
			ip_prefix varchar(64) NOT NULL DEFAULT '',
			last_seen_at timestamptz NOT NULL DEFAULT now(),
			family_started_at timestamptz NOT NULL DEFAULT now()
		)`,
		`ALTER TABLE public.refresh_sessions ADD COLUMN IF NOT EXISTS client_label varchar(120) NOT NULL DEFAULT ''`,
		`ALTER TABLE public.refresh_sessions ADD COLUMN IF NOT EXISTS ip_prefix varchar(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE public.refresh_sessions ADD COLUMN IF NOT EXISTS last_seen_at timestamptz NOT NULL DEFAULT now()`,
		`ALTER TABLE public.refresh_sessions ADD COLUMN IF NOT EXISTS family_started_at timestamptz NOT NULL DEFAULT now()`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func seedUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	id := newID()
	// uuid format for postgres uuid column
	uid := id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:32]
	_, err := pool.Exec(context.Background(),
		`INSERT INTO public.users (id, name, email, password) VALUES ($1::uuid, 't', $2, 'x')`,
		uid, uid+"@example.com")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return uid
}

func session(userID, familyID, tokenID string, hash [32]byte, now time.Time) domain.RefreshSession {
	return domain.RefreshSession{
		TokenID:   tokenID,
		FamilyID:  familyID,
		UserID:    userID,
		TokenHash: hash,
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}
}

func hash32(seed byte) [32]byte {
	var h [32]byte
	_, _ = rand.Read(h[:])
	h[0] = seed // keep a distinguisher for debugging; rest is random
	return h
}

func cleanupSessions(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE public.refresh_sessions`); err != nil {
		t.Fatalf("truncate refresh_sessions: %v", err)
	}
}

func TestRotateConcurrentExactlyOneWins(t *testing.T) {
	pool := testPool(t)
	setupSchema(t, pool)
	cleanupSessions(t, pool)
	userID := seedUser(t, pool)
	db := database.NewPostgresPgxForTest(pool)
	store := refreshstore.New(db)
	ctx := context.Background()
	now := time.Now().UTC()
	family := newID()
	currentHash := hash32(1)

	if err := store.Create(ctx, session(userID, family, newID(), currentHash, now)); err != nil {
		t.Fatalf("create: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			nextHash := hash32(byte(10 + i))
			results <- store.Rotate(ctx, currentHash, session(userID, family, newID(), nextHash, now))
		}(i)
	}
	wg.Wait()
	close(results)

	var wins, consumed int
	for err := range results {
		switch {
		case err == nil:
			wins++
		case err == core.ErrRefreshConsumed:
			consumed++
		default:
			t.Fatalf("unexpected err: %v", err)
		}
	}
	if wins != 1 || consumed != 1 {
		t.Fatalf("wins=%d consumed=%d want 1/1", wins, consumed)
	}
}

func TestReplayAfterConsumeRevokeFamilyIdempotent(t *testing.T) {
	pool := testPool(t)
	setupSchema(t, pool)
	cleanupSessions(t, pool)
	userID := seedUser(t, pool)
	store := refreshstore.New(database.NewPostgresPgxForTest(pool))
	ctx := context.Background()
	now := time.Now().UTC()
	family := newID()
	h1 := hash32(20)
	h2 := hash32(21)

	if err := store.Create(ctx, session(userID, family, newID(), h1, now)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := store.Rotate(ctx, h1, session(userID, family, newID(), h2, now)); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if err := store.Rotate(ctx, h1, session(userID, family, newID(), hash32(22), now)); err != core.ErrRefreshConsumed {
		t.Fatalf("replay want ErrRefreshConsumed got %v", err)
	}
	if err := store.RevokeFamily(ctx, family); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := store.RevokeFamily(ctx, family); err != nil {
		t.Fatalf("revoke idempotent: %v", err)
	}
	if err := store.Rotate(ctx, h2, session(userID, family, newID(), hash32(23), now)); err != core.ErrRefreshFamilyRevoked {
		t.Fatalf("after revoke want ErrRefreshFamilyRevoked got %v", err)
	}
}

func TestRevokeUserRevokesAllFamiliesAndIsolatesUsers(t *testing.T) {
	pool := testPool(t)
	setupSchema(t, pool)
	cleanupSessions(t, pool)
	userA := seedUser(t, pool)
	userB := seedUser(t, pool)
	store := refreshstore.New(database.NewPostgresPgxForTest(pool))
	ctx := context.Background()
	now := time.Now().UTC()

	a1 := session(userA, newID(), newID(), hash32(31), now)
	a2 := session(userA, newID(), newID(), hash32(32), now)
	b1 := session(userB, newID(), newID(), hash32(33), now)
	for _, item := range []domain.RefreshSession{a1, a2, b1} {
		if err := store.Create(ctx, item); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	if err := store.RevokeUser(ctx, userA); err != nil {
		t.Fatalf("RevokeUser() error = %v", err)
	}
	if err := store.RevokeUser(ctx, userA); err != nil {
		t.Fatalf("RevokeUser() idempotent error = %v", err)
	}

	var revokedA, revokedB int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM public.refresh_sessions WHERE user_id = $1 AND revoked_at IS NOT NULL`,
		userA,
	).Scan(&revokedA); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM public.refresh_sessions WHERE user_id = $1 AND revoked_at IS NOT NULL`,
		userB,
	).Scan(&revokedB); err != nil {
		t.Fatal(err)
	}
	if revokedA != 2 || revokedB != 0 {
		t.Fatalf("revoked userA=%d userB=%d, want 2/0", revokedA, revokedB)
	}

	if err := store.Rotate(ctx, b1.TokenHash, session(userB, b1.FamilyID, newID(), hash32(34), now)); err != nil {
		t.Fatalf("other user Rotate() error = %v", err)
	}
}

func TestPurgeExpiredHonorsCutoffAndBatchLimit(t *testing.T) {
	pool := testPool(t)
	setupSchema(t, pool)
	cleanupSessions(t, pool)
	userID := seedUser(t, pool)
	store := refreshstore.NewWithPurgeBatchSize(database.NewPostgresPgxForTest(pool), 2)
	ctx := context.Background()
	cutoff := time.Now().UTC().Truncate(time.Second)

	sessions := []domain.RefreshSession{
		{TokenID: newID(), FamilyID: newID(), UserID: userID, TokenHash: hash32(41), IssuedAt: cutoff.Add(-4 * time.Hour), ExpiresAt: cutoff.Add(-3 * time.Hour)},
		{TokenID: newID(), FamilyID: newID(), UserID: userID, TokenHash: hash32(42), IssuedAt: cutoff.Add(-3 * time.Hour), ExpiresAt: cutoff.Add(-2 * time.Hour)},
		{TokenID: newID(), FamilyID: newID(), UserID: userID, TokenHash: hash32(43), IssuedAt: cutoff.Add(-2 * time.Hour), ExpiresAt: cutoff.Add(-time.Hour)},
		{TokenID: newID(), FamilyID: newID(), UserID: userID, TokenHash: hash32(44), IssuedAt: cutoff.Add(-time.Hour), ExpiresAt: cutoff},
		{TokenID: newID(), FamilyID: newID(), UserID: userID, TokenHash: hash32(45), IssuedAt: cutoff, ExpiresAt: cutoff.Add(time.Hour)},
	}
	for _, item := range sessions {
		if err := store.Create(ctx, item); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	deleted, err := store.PurgeExpired(ctx, cutoff)
	if err != nil {
		t.Fatalf("PurgeExpired() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("first PurgeExpired() deleted = %d, want 2", deleted)
	}
	deleted, err = store.PurgeExpired(ctx, cutoff)
	if err != nil {
		t.Fatalf("second PurgeExpired() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("second PurgeExpired() deleted = %d, want 1", deleted)
	}

	var remaining, boundary int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.refresh_sessions WHERE user_id = $1`, userID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM public.refresh_sessions WHERE user_id = $1 AND expires_at = $2`,
		userID,
		cutoff,
	).Scan(&boundary); err != nil {
		t.Fatal(err)
	}
	if remaining != 2 || boundary != 1 {
		t.Fatalf("remaining=%d boundary=%d, want 2/1", remaining, boundary)
	}
}

func TestRevokeUserRacesSafelyWithRotate(t *testing.T) {
	pool := testPool(t)
	setupSchema(t, pool)
	cleanupSessions(t, pool)
	userID := seedUser(t, pool)
	store := refreshstore.New(database.NewPostgresPgxForTest(pool))
	ctx := context.Background()
	now := time.Now().UTC()
	current := session(userID, newID(), newID(), hash32(51), now)
	if err := store.Create(ctx, current); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		results <- store.Rotate(ctx, current.TokenHash, session(userID, current.FamilyID, newID(), hash32(52), now))
	}()
	go func() {
		<-start
		results <- store.RevokeUser(ctx, userID)
	}()
	close(start)

	for i := 0; i < 2; i++ {
		err := <-results
		if err != nil && !errors.Is(err, core.ErrRefreshFamilyRevoked) {
			t.Fatalf("race operation error = %v", err)
		}
	}

	var active int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM public.refresh_sessions WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
	).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("active sessions after revoke/rotate race = %d, want 0", active)
	}
}

func TestListActiveByUserIsolatesAndHidesInactive(t *testing.T) {
	pool := testPool(t)
	setupSchema(t, pool)
	cleanupSessions(t, pool)
	userA := seedUser(t, pool)
	userB := seedUser(t, pool)
	db := database.NewPostgresPgxForTest(pool)
	store := refreshstore.New(db)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := store.Create(ctx, session(userA, "family-a", newID(), hash32(1), now)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, session(userB, "family-b", newID(), hash32(2), now)); err != nil {
		t.Fatal(err)
	}
	expired := session(userA, "family-expired", newID(), hash32(3), now.Add(-2*time.Hour))
	expired.ExpiresAt = now.Add(-time.Minute)
	if err := store.Create(ctx, expired); err != nil {
		t.Fatal(err)
	}

	page, err := store.ListActiveByUser(ctx, userA, domain.PageRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].FamilyID != "family-a" {
		t.Fatalf("items=%+v", page.Items)
	}

	if err := store.RevokeFamilyForUser(ctx, userB, "family-a"); err != nil {
		t.Fatal(err)
	}
	page, err = store.ListActiveByUser(ctx, userA, domain.PageRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("foreign revoke leaked: %+v", page.Items)
	}

	if err := store.RevokeFamilyForUser(ctx, userA, "family-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeFamilyForUser(ctx, userA, "family-a"); err != nil {
		t.Fatal(err)
	}
	page, err = store.ListActiveByUser(ctx, userA, domain.PageRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("revoked still listed: %+v", page.Items)
	}
}

func TestListActiveByUserKeysetCursor(t *testing.T) {
	pool := testPool(t)
	setupSchema(t, pool)
	cleanupSessions(t, pool)
	userID := seedUser(t, pool)
	db := database.NewPostgresPgxForTest(pool)
	store := refreshstore.New(db)
	ctx := context.Background()
	now := time.Now().UTC()
	for i := byte(0); i < 3; i++ {
		if err := store.Create(ctx, session(userID, fmt.Sprintf("family-%d", i), newID(), hash32(i), now.Add(time.Duration(i)*time.Millisecond))); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.ListActiveByUser(ctx, userID, domain.PageRequest{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || !first.HasNext {
		t.Fatalf("first page=%+v", first)
	}
	last := first.Items[len(first.Items)-1]
	second, err := store.ListActiveByUser(ctx, userID, domain.PageRequest{
		Limit: 2,
		After: &domain.CursorKey{CreatedAt: last.LastSeenAt, ID: last.FamilyID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.HasNext {
		t.Fatalf("second page=%+v", second)
	}
	if second.Items[0].FamilyID == first.Items[0].FamilyID || second.Items[0].FamilyID == first.Items[1].FamilyID {
		t.Fatalf("cursor overlapped %+v %+v", first.Items, second.Items)
	}
}
