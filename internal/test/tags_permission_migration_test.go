package testdb_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gregperez/hachikapp-backend/internal/test/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

type adminRoleCounts struct {
	tagsRoutes, customRoutes, tagsPerms, tagsCreateRead, customPerms int
}

func countAdminRole(t *testing.T, pool *pgxpool.Pool) adminRoleCounts {
	t.Helper()
	var c adminRoleCounts
	err := pool.QueryRow(context.Background(), `
SELECT
  cardinality(array_positions(routes, 'TAGS')),
  cardinality(array_positions(routes, 'CUSTOM_TEST')),
  (SELECT count(*) FROM jsonb_array_elements(permissions) p WHERE p->>'entity' = 'TAGS'),
  (SELECT count(*) FROM jsonb_array_elements(permissions) p
     WHERE p @> '{"entity":"TAGS","create":true,"read":true,"update":false,"delete":false}'),
  (SELECT count(*) FROM jsonb_array_elements(permissions) p WHERE p->>'entity' = 'CUSTOM_TEST')
FROM public.roles WHERE name = 'admin'`).Scan(&c.tagsRoutes, &c.customRoutes, &c.tagsPerms, &c.tagsCreateRead, &c.customPerms)
	if err != nil {
		t.Fatalf("read admin role: %v", err)
	}
	return c
}

func execMigrationFile(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()
	sqlText, err := os.ReadFile(filepath.Join(migrationsPath(t), name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), string(sqlText)); err != nil {
		t.Fatalf("re-run %s: %v", name, err)
	}
}

// 000020 must only add (and remove) the TAGS route and permission, keeping
// any other change made to the admin role, and be safe to run twice.
func TestTagsPermissionMigrationIsAdditive(t *testing.T) {
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

	stripCustom := func() {
		_, _ = pool.Exec(context.Background(), `
UPDATE public.roles
SET routes = array_remove(routes, 'CUSTOM_TEST'),
    permissions = COALESCE((SELECT jsonb_agg(p) FROM jsonb_array_elements(permissions) p
                            WHERE p->>'entity' <> 'CUSTOM_TEST'), '[]'::jsonb)
WHERE name = 'admin'`)
	}
	migrateTo(t, dsn, "19")
	stripCustom()
	t.Cleanup(func() {
		stripCustom()
		migrateTo(t, dsn, "20")
	})

	if _, err := pool.Exec(ctx, `
UPDATE public.roles
SET routes = routes || ARRAY['CUSTOM_TEST']::text[],
    permissions = permissions || '[{"entity":"CUSTOM_TEST","create":false,"read":true,"update":false,"delete":false}]'::jsonb
WHERE name = 'admin'`); err != nil {
		t.Fatalf("customize admin: %v", err)
	}

	migrateTo(t, dsn, "20")
	want := adminRoleCounts{tagsRoutes: 1, customRoutes: 1, tagsPerms: 1, tagsCreateRead: 1, customPerms: 1}
	if got := countAdminRole(t, pool); got != want {
		t.Fatalf("after up: got %+v want %+v", got, want)
	}
	execMigrationFile(t, pool, "000020_tags_permission.up.sql")
	if got := countAdminRole(t, pool); got != want {
		t.Fatalf("after second up: got %+v want %+v", got, want)
	}

	migrateTo(t, dsn, "19")
	want = adminRoleCounts{customRoutes: 1, customPerms: 1}
	if got := countAdminRole(t, pool); got != want {
		t.Fatalf("after down: got %+v want %+v", got, want)
	}
	execMigrationFile(t, pool, "000020_tags_permission.down.sql")
	if got := countAdminRole(t, pool); got != want {
		t.Fatalf("after second down: got %+v want %+v", got, want)
	}
}
