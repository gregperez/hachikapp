package testdb_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/test/testdb"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openPool(t *testing.T) (*pgxpool.Pool, context.Context) {
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

	var hardened bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname = 'roles_name_key')`).Scan(&hardened); err != nil {
		t.Fatal(err)
	}
	if !hardened {
		t.Skip("migration 000009 not applied (run make test-db / migrate-verify first)")
	}
	return pool, ctx
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isNotNullViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23502"
}

func TestRolesNameUnique(t *testing.T) {
	pool, ctx := openPool(t)
	name := "dup-role-" + time.Now().Format("150405.000")
	if _, err := pool.Exec(ctx, `INSERT INTO public.roles (name) VALUES ($1)`, name); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO public.roles (name) VALUES ($1)`, name)
	if !isUniqueViolation(err) {
		t.Fatalf("second insert err=%v, want unique_violation", err)
	}
}

func TestUsersRolesRejectsNullAndDuplicate(t *testing.T) {
	pool, ctx := openPool(t)

	var userID, roleID string
	email := "schema-" + time.Now().Format("150405.000") + "@example.com"
	if err := pool.QueryRow(ctx,
		`INSERT INTO public.users (name, email, password) VALUES ('t', $1, 'x') RETURNING id`, email,
	).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO public.roles (name) VALUES ($1) RETURNING id`, "r-"+time.Now().Format("150405.000"),
	).Scan(&roleID); err != nil {
		t.Fatal(err)
	}

	_, err := pool.Exec(ctx, `INSERT INTO public.users_roles (user_id, role_id) VALUES (NULL, $1)`, roleID)
	if !isNotNullViolation(err) {
		t.Fatalf("null user_id err=%v, want not_null_violation", err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO public.users_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO public.users_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID)
	if !isUniqueViolation(err) {
		t.Fatalf("duplicate pair err=%v, want unique_violation", err)
	}
}

func TestEntityTimestampsAreManagedByDatabase(t *testing.T) {
	pool, ctx := openPool(t)

	email := "timestamps-" + time.Now().Format("150405.000") + "@example.com"
	var userID string
	var createdAt, updatedAt time.Time
	if err := pool.QueryRow(ctx, `
INSERT INTO public.users (name, email, password)
VALUES ('timestamps', $1, 'x')
RETURNING id, created_at, updated_at`, email).Scan(&userID, &createdAt, &updatedAt); err != nil {
		t.Fatal(err)
	}
	if createdAt.IsZero() || updatedAt.IsZero() || !createdAt.Equal(updatedAt) {
		t.Fatalf("insert timestamps created=%v updated=%v", createdAt, updatedAt)
	}

	var createdAfter, updatedAfter time.Time
	if err := pool.QueryRow(ctx, `
UPDATE public.users
SET name = 'timestamps-updated',
    created_at = created_at - interval '1 day'
WHERE id = $1
RETURNING created_at, updated_at`, userID).Scan(&createdAfter, &updatedAfter); err != nil {
		t.Fatal(err)
	}
	if !createdAfter.Equal(createdAt) {
		t.Fatalf("created_at changed: before=%v after=%v", createdAt, createdAfter)
	}
	if !updatedAfter.After(updatedAt) {
		t.Fatalf("updated_at did not advance: before=%v after=%v", updatedAt, updatedAfter)
	}

	var roleID string
	var roleCreatedAt, roleUpdatedAt time.Time
	if err := pool.QueryRow(ctx, `
INSERT INTO public.roles (name)
VALUES ($1)
RETURNING id, created_at, updated_at`,
		"timestamps-"+time.Now().Format("150405.000"),
	).Scan(&roleID, &roleCreatedAt, &roleUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if roleCreatedAt.IsZero() || roleUpdatedAt.IsZero() {
		t.Fatal("role timestamps were not initialized")
	}

	var assignmentCreatedAt, assignmentUpdatedAt time.Time
	if err := pool.QueryRow(ctx, `
INSERT INTO public.users_roles (user_id, role_id)
VALUES ($1, $2)
RETURNING created_at, updated_at`, userID, roleID).Scan(&assignmentCreatedAt, &assignmentUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if assignmentCreatedAt.IsZero() || assignmentUpdatedAt.IsZero() {
		t.Fatal("users_roles timestamps were not initialized")
	}
}

func TestUsersRolesCascadeOnUserDelete(t *testing.T) {
	pool, ctx := openPool(t)
	var userID, roleID string
	email := "cascade-" + time.Now().Format("150405.000") + "@example.com"
	if err := pool.QueryRow(ctx,
		`INSERT INTO public.users (name, email, password) VALUES ('t', $1, 'x') RETURNING id`, email,
	).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO public.roles (name) VALUES ($1) RETURNING id`, "rc-"+time.Now().Format("150405.000"),
	).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.users_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM public.users WHERE id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.users_roles WHERE role_id = $1`, roleID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected cascade delete of junction rows, remaining=%d", n)
	}
}

func TestLoginEmailUsesUniqueIndex(t *testing.T) {
	pool, ctx := openPool(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, `EXPLAIN SELECT id FROM public.users WHERE email = $1`, "nobody@example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	plan := strings.ToLower(b.String())
	if !strings.Contains(plan, "index") && !strings.Contains(plan, "unique") {
		t.Fatalf("expected index usage hint in plan, got:\n%s", b.String())
	}
}

func TestExternalIdentitiesUniqueIssuerSubject(t *testing.T) {
	pool, ctx := openPool(t)
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname = 'external_identities_issuer_subject_key')`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Skip("migration 000013 not applied")
	}

	var userID string
	email := "oidc-" + time.Now().Format("150405.000") + "@example.com"
	if err := pool.QueryRow(ctx,
		`INSERT INTO public.users (name, email, password) VALUES ('t', $1, 'x') RETURNING id`, email,
	).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO public.external_identities (provider, issuer, subject, user_id)
VALUES ('google', 'https://accounts.example.test', 'sub-dup', $1)`, userID); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `
INSERT INTO public.external_identities (provider, issuer, subject, user_id)
VALUES ('google', 'https://accounts.example.test', 'sub-dup', $1)`, userID)
	if !isUniqueViolation(err) {
		t.Fatalf("second identity err=%v, want unique_violation", err)
	}
}

func TestSchemaHasHardenedConstraints(t *testing.T) {
	pool, ctx := openPool(t)
	for _, name := range []string{"roles_name_key", "users_roles_user_id_role_id_key"} {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname = $1)`, name).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("%s missing — apply migration 000009", name)
		}
	}
}
