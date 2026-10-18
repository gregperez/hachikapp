package domain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

type testBlocklist map[string]struct{}

func (b testBlocklist) Blocked(ctx context.Context, password string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	_, blocked := b[password]
	return blocked, nil
}

func TestValidatePasswordContext(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		pwd     string
		terms   []string
		wantErr bool
	}{
		{name: "14 runes", pwd: strings.Repeat("a", 14), wantErr: true},
		{name: "15 runes", pwd: strings.Repeat("a", 15)},
		{name: "64 runes", pwd: strings.Repeat("x", 64)},
		{name: "128 runes", pwd: strings.Repeat("z", 128)},
		{name: "129 runes", pwd: strings.Repeat("z", 129), wantErr: true},
		{name: "unicode counted as runes", pwd: strings.Repeat("界", 15)},
		{name: "spaces allowed", pwd: "  a long pass phrase  "},
		{name: "common password", pwd: "correct horse battery staple", wantErr: true},
		{name: "context name", pwd: "safe-ada-lovelace-passphrase", terms: []string{"Ada Lovelace"}, wantErr: true},
		{name: "context email", pwd: "something-wilson-safe", terms: []string{"wilson@example.com"}, wantErr: true},
		{name: "oversized bytes", pwd: strings.Repeat("界", 128) + strings.Repeat("\u0301", 700), wantErr: true},
	}
	blocklist := testBlocklist{"correct horse battery staple": {}}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := domain.ValidatePasswordContext(context.Background(), tc.pwd, blocklist, tc.terms...)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePasswordContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := domain.ValidatePasswordContext(ctx, strings.Repeat("a", 15), testBlocklist{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
}
