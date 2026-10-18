package core

import (
	"context"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
)

// AuditUseCase lists sanitized administrative audit events.
type AuditUseCase interface {
	List(ctx context.Context, filter domain.AuditListFilter, request authBaseDomain.PageRequest) (authBaseDomain.Page[domain.AuditEvent], error)
}
