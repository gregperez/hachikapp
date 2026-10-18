package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/services"
)

type memAuditEvents struct {
	items []domain.AuditEvent
	err   error
}

func (m *memAuditEvents) Insert(context.Context, domain.AuditEvent) error { return nil }

func (m *memAuditEvents) List(_ context.Context, _ domain.AuditListFilter, _ authBaseDomain.PageRequest) (authBaseDomain.Page[domain.AuditEvent], error) {
	if m.err != nil {
		return authBaseDomain.Page[domain.AuditEvent]{}, m.err
	}
	return authBaseDomain.Page[domain.AuditEvent]{Items: m.items}, nil
}

func (m *memAuditEvents) PurgeBefore(context.Context, time.Time) (int64, error) { return 0, nil }

func TestAuditServiceRejectsUnknownAction(t *testing.T) {
	t.Parallel()
	svc := services.NewAuditService(&memAuditEvents{})
	_, err := svc.List(context.Background(), domain.AuditListFilter{Action: "drop table"}, authBaseDomain.PageRequest{Limit: 10})
	if !errors.Is(err, domain.ErrInvalidAuditFilter) {
		t.Fatalf("err=%v", err)
	}
}

func TestAuditServiceListsKnownAction(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	svc := services.NewAuditService(&memAuditEvents{items: []domain.AuditEvent{{
		ID: "1", Action: "login_success", OccurredAt: now, Category: domain.AuditCategorySecurity, Outcome: "success", ActorID: "system",
	}}})
	page, err := svc.List(context.Background(), domain.AuditListFilter{Action: "login_success"}, authBaseDomain.PageRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items=%d", len(page.Items))
	}
}
