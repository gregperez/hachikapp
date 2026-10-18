package testdb_test

import (
	"os"
	"testing"

	"github.com/WilsonSayago/base-golang/internal/test/testdb"
)

func TestRequireURLRejectsUnsafe(t *testing.T) {
	t.Setenv("DATABASE_TEST_URL", "postgres://u:p@db.prod.example/app?sslmode=require")
	if _, err := testdb.RequireURL(); err == nil {
		t.Fatal("expected rejection for remote host")
	}

	t.Setenv("DATABASE_TEST_URL", "postgres://u:p@localhost/postgres?sslmode=disable")
	if _, err := testdb.RequireURL(); err == nil {
		t.Fatal("expected rejection for database name without test")
	}
}

func TestRequireURLAcceptsLocalTestDB(t *testing.T) {
	t.Setenv("DATABASE_TEST_URL", "postgres://postgres:test@localhost:54329/base_golang_test?sslmode=disable")
	got, err := testdb.RequireURL()
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("empty url")
	}
	_ = os.Getenv // keep import used if build tags change
}
