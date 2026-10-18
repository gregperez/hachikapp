package core

import (
	"context"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// AuditUseCase lists sanitized administrative audit events.
type AuditUseCase interface {
	List(ctx context.Context, filter domain.AuditListFilter, request authBaseDomain.PageRequest) (authBaseDomain.Page[domain.AuditEvent], error)
}
