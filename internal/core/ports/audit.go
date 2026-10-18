package ports

import (
	"context"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// PrivilegeAuditor records privileged mutations without secrets or PII.
type PrivilegeAuditor interface {
	PrivilegeChange(ctx context.Context, action, actorID, subjectID string, roleIDs []string, isAdmin *bool)
}

// AuditEventPort persists and lists sanitized audit events.
type AuditEventPort interface {
	Insert(ctx context.Context, event domain.AuditEvent) error
	List(ctx context.Context, filter domain.AuditListFilter, request authBaseDomain.PageRequest) (authBaseDomain.Page[domain.AuditEvent], error)
	PurgeBefore(ctx context.Context, before time.Time) (int64, error)
}

// NoopAuditor discards audit events (tests).
type NoopAuditor struct{}

func (NoopAuditor) PrivilegeChange(context.Context, string, string, string, []string, *bool) {}
