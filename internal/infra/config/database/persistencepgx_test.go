package database

import (
	"testing"

	"github.com/WilsonSayago/base-golang/internal/infra/config/prop"
)

func TestNewPostgresPgxForTest_WrapsPool(t *testing.T) {
	t.Parallel()
	// Nil pool is allowed for construction smoke test of the wrapper pointer.
	db := NewPostgresPgxForTest(nil)
	if db == nil {
		t.Fatal("expected non-nil wrapper")
	}
	if db.Pool() != nil {
		t.Fatal("expected nil pool passthrough")
	}
}

func TestDatabasePropertyDefaults(t *testing.T) {
	t.Parallel()
	p := prop.NewDatabaseProperty()
	if p.Host != "" || p.Password != "" {
		t.Fatalf("unexpected defaults: %+v", p)
	}
}
