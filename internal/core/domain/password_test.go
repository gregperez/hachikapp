package domain_test

import (
	"testing"

	"github.com/WilsonSayago/base-golang/internal/core/domain"
)

func TestValidatePassword(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		pwd     string
		wantErr bool
	}{
		{name: "ok", pwd: "CorrectHorse1", wantErr: false},
		{name: "too short", pwd: "Short1Aa", wantErr: true},
		{name: "no upper", pwd: "correcthorse1", wantErr: true},
		{name: "no lower", pwd: "CORRECTHORSE1", wantErr: true},
		{name: "no digit", pwd: "CorrectHorse!", wantErr: true},
		{name: "leading space", pwd: " CorrectHorse1", wantErr: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := domain.ValidatePassword(tc.pwd)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
