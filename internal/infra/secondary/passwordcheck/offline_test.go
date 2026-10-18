package passwordcheck_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/passwordcheck"
)

func TestOfflineNormalizesAndHonorsCancellation(t *testing.T) {
	t.Parallel()
	checker := passwordcheck.NewOffline([]string{"Correct Horse Battery Staple"})
	blocked, err := checker.Blocked(context.Background(), "  correct horse battery staple  ")
	if err != nil || !blocked {
		t.Fatalf("blocked=%v err=%v", blocked, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = checker.Blocked(ctx, "anything")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}
