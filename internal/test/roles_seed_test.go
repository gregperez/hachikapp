package testdb_test

import (
	"context"
	"strings"
	"testing"

	"github.com/WilsonSayago/base-golang/internal/test/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTightenDefaultRolesSeed(t *testing.T) {
	dsn, err := testdb.RequireURL()
	if err != nil {
		t.Skip(err.Error())
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(ctx, `SELECT name, permissions::text FROM public.roles WHERE name IN ('admin','user','guest') ORDER BY name`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	perms := map[string]string{}
	for rows.Next() {
		var name, p string
		if err := rows.Scan(&name, &p); err != nil {
			t.Fatal(err)
		}
		perms[name] = p
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if _, ok := perms["admin"]; !ok {
		t.Fatal("admin role missing — run migrations first")
	}
	for _, name := range []string{"user", "guest"} {
		p := perms[name]
		if strings.Contains(p, `"create":true`) || strings.Contains(p, `"update":true`) || strings.Contains(p, `"delete":true`) {
			t.Fatalf("role %s still has admin CRUD: %s", name, p)
		}
	}
	admin := perms["admin"]
	for _, needle := range []string{"USERS", "ROLES", "USERS_PRIVILEGES"} {
		if !strings.Contains(admin, needle) {
			t.Fatalf("admin missing %s in %s", needle, admin)
		}
	}
}
