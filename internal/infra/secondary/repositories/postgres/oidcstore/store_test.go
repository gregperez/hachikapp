package oidcstore_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/oidcstore"
	"github.com/gregperez/hachikapp-backend/internal/test/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openStore(t *testing.T) (*oidcstore.Store, *pgxpool.Pool, context.Context) {
	t.Helper()
	dsn, err := testdb.RequireURL()
	if err != nil {
		t.Skip(err.Error())
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var ready bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'oidc_transactions')`).Scan(&ready); err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Skip("migration 000013 not applied")
	}
	return oidcstore.New(database.NewPostgresPgxForTest(pool)), pool, ctx
}

func insertUser(t *testing.T, pool *pgxpool.Pool, ctx context.Context) string {
	t.Helper()
	var id string
	email := "oidc-store-" + time.Now().Format("150405.000000") + "@example.com"
	if err := pool.QueryRow(ctx, `INSERT INTO public.users (name, email, password) VALUES ('t', $1, 'x') RETURNING id`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestConsumeStateIsSingleUse(t *testing.T) {
	store, _, ctx := openStore(t)
	hash := []byte("state-hash-" + time.Now().Format("150405.000000"))
	if err := store.Create(ctx, domain.OIDCTransaction{
		Provider:       "google",
		StateHash:      hash,
		Nonce:          []byte("nonce"),
		VerifierCipher: []byte("cipher"),
		Intent:         domain.OIDCIntentLogin,
		ReturnTo:       "https://app.example.com/auth/oidc",
		ExpiresAt:      time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := store.ConsumeByStateHash(ctx, hash, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeByStateHash(ctx, hash, now); err != domain.ErrOIDCReplay {
		t.Fatalf("err=%v", err)
	}
}

func TestConcurrentConsumeState(t *testing.T) {
	store, _, ctx := openStore(t)
	hash := []byte("state-race-" + time.Now().Format("150405.000000"))
	if err := store.Create(ctx, domain.OIDCTransaction{
		Provider:       "google",
		StateHash:      hash,
		Nonce:          []byte("nonce"),
		VerifierCipher: []byte("cipher"),
		Intent:         domain.OIDCIntentLogin,
		ReturnTo:       "https://app.example.com/auth/oidc",
		ExpiresAt:      time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	now := time.Now()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.ConsumeByStateHash(ctx, hash, now)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	var ok, replay int
	for err := range errs {
		switch err {
		case nil:
			ok++
		case domain.ErrOIDCReplay:
			replay++
		default:
			t.Fatalf("unexpected err=%v", err)
		}
	}
	if ok != 1 || replay != 1 {
		t.Fatalf("ok=%d replay=%d", ok, replay)
	}
}

func TestConsumeExpiredGrant(t *testing.T) {
	store, pool, ctx := openStore(t)
	hash := []byte("state-expired-grant-" + time.Now().Format("150405.000000"))
	if err := store.Create(ctx, domain.OIDCTransaction{
		Provider:       "google",
		StateHash:      hash,
		Nonce:          []byte("nonce"),
		VerifierCipher: []byte("cipher"),
		Intent:         domain.OIDCIntentLogin,
		ReturnTo:       "https://app.example.com/auth/oidc",
		ExpiresAt:      time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	tx, err := store.ConsumeByStateHash(ctx, hash, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	grant := []byte("grant-hash-" + time.Now().Format("150405.000000"))
	if err := store.AttachGrant(ctx, tx.ID, grant, insertUser(t, pool, ctx), time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeGrant(ctx, grant, time.Now()); err != domain.ErrOIDCExpired {
		t.Fatalf("err=%v", err)
	}
}

func TestConcurrentLinkConflict(t *testing.T) {
	store, pool, ctx := openStore(t)
	userA := insertUser(t, pool, ctx)
	userB := insertUser(t, pool, ctx)
	subject := "sub-race-" + time.Now().Format("150405.000000")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, userID := range []string{userA, userB} {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			errs <- store.Link(ctx, domain.ExternalIdentity{
				Provider: "google", Issuer: "https://accounts.example.test", Subject: subject, UserID: userID,
			})
		}(userID)
	}
	wg.Wait()
	close(errs)
	var ok, conflict int
	for err := range errs {
		switch err {
		case nil:
			ok++
		case domain.ErrConflict:
			conflict++
		default:
			t.Fatalf("unexpected err=%v", err)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("ok=%d conflict=%d", ok, conflict)
	}
}

func TestLinkConflictOnIssuerSubject(t *testing.T) {
	store, pool, ctx := openStore(t)
	userA := insertUser(t, pool, ctx)
	userB := insertUser(t, pool, ctx)
	subject := "sub-" + time.Now().Format("150405.000000")
	if err := store.Link(ctx, domain.ExternalIdentity{
		Provider: "google", Issuer: "https://accounts.example.test", Subject: subject, UserID: userA,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Link(ctx, domain.ExternalIdentity{
		Provider: "google", Issuer: "https://accounts.example.test", Subject: subject, UserID: userB,
	}); err != domain.ErrConflict {
		t.Fatalf("err=%v", err)
	}
}
