package prop

import "testing"

func TestNewDatabaseProperty(t *testing.T) {
	t.Parallel()
	got := NewDatabaseProperty()
	if got.Host != "" || got.Password != "" {
		t.Fatalf("expected empty defaults, got %+v", got)
	}
}
