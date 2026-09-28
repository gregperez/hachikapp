package audit_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/audit"
)

type failStore struct{}

func (failStore) Insert(context.Context, domain.AuditEvent) error {
	return errors.New("store down")
}

func (failStore) List(context.Context, domain.AuditListFilter, authBaseDomain.PageRequest) (authBaseDomain.Page[domain.AuditEvent], error) {
	return authBaseDomain.Page[domain.AuditEvent]{}, nil
}

func (failStore) PurgeBefore(context.Context, time.Time) (int64, error) { return 0, nil }

func TestRecorderPersistFailureDoesNotPanicAndLogs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	rec, err := audit.NewRecorder(slog.New(slog.NewJSONHandler(&buf, nil)), failStore{})
	if err != nil {
		t.Fatal(err)
	}
	rec.Record(context.Background(), authBaseCore.SecurityEvent{
		Type:    authBaseCore.SecurityLoginSuccess,
		ActorID: "user-1",
		Outcome: "success",
	})
	body := buf.String()
	if !strings.Contains(body, "audit persist failed") {
		t.Fatalf("missing failure log: %s", body)
	}
	if strings.Contains(strings.ToLower(body), "password") {
		t.Fatalf("leaked secret: %s", body)
	}
}
