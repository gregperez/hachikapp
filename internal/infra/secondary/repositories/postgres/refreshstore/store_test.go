package refreshstore_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/WilsonSayago/authBase/v3/core"
	"github.com/WilsonSayago/authBase/v3/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/refreshstore"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_TEST_URL")
	if dsn == "" {
		t.Skip("DATABASE_TEST_URL not set; ephemeral Postgres required for refresh store atomicity tests")
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
			revoked_at timestamptz NULL
		)`,
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
