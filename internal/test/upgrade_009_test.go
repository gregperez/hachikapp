package testdb_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSayago/base-golang/internal/test/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func migrationsPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "internal", "resources", "sql", "migrations"))
}

func migrateTo(t *testing.T, dsn, version string) {
	t.Helper()
	cmd := exec.Command("go", "run", "-tags", "postgres",
		"github.com/golang-migrate/migrate/v4/cmd/migrate@v4.20.1",
		"-path", migrationsPath(t),
		"-database", dsn,
		"goto", version,
	)
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migrate goto %s: %v\n%s", version, err, out)
	}
}

func TestHardenUpgradeAbortsDivergentRoleDuplicates(t *testing.T) {
	dsn, err := testdb.RequireURL()
	if err != nil {
		t.Skip(err.Error())
	}
	ctx := context.Background()

	// Reset to version 8 (before harden).
	migrateTo(t, dsn, "8")
	t.Cleanup(func() {
		pool, err := pgxpool.New(context.Background(), dsn)
		if err == nil {
			_, _ = pool.Exec(context.Background(), `DELETE FROM public.roles WHERE name LIKE 'divergent-%'`)
			pool.Close()
		}
		cmd := exec.Command("go", "run", "-tags", "postgres",
			"github.com/golang-migrate/migrate/v4/cmd/migrate@v4.20.1",
			"-path", migrationsPath(t),
			"-database", dsn,
			"force", "8",
		)
		cmd.Env = append(os.Environ(), "GOWORK=off")
		_ = cmd.Run()
		migrateTo(t, dsn, "12")
	})

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	name := "divergent-" + time.Now().Format("150405.000")
	_, err = pool.Exec(ctx, `
INSERT INTO public.roles (name, routes, permissions) VALUES
 ($1, ARRAY['USERS'], '[{"entity":"USERS","read":true}]'::jsonb),
 ($1, ARRAY['ROLES'], '[{"entity":"ROLES","read":true}]'::jsonb)
`, name)
	if err != nil {
		t.Fatalf("seed divergent duplicates: %v", err)
	}

	cmd := exec.Command("go", "run", "-tags", "postgres",
		"github.com/golang-migrate/migrate/v4/cmd/migrate@v4.20.1",
		"-path", migrationsPath(t),
		"-database", dsn,
		"up", "1",
	)
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected 000009 to abort on divergent duplicates; out=%s", out)
	}
	if !strings.Contains(string(out), "divergent") && !strings.Contains(string(out), "000009") {
		t.Fatalf("unexpected migrate error output: %s", out)
	}
}
