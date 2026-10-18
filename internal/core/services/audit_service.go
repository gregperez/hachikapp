package services

import (
	"context"
	"fmt"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
)

const maxAuditRange = 90 * 24 * time.Hour

type AuditService struct {
	events ports.AuditEventPort
}

func NewAuditService(events ports.AuditEventPort) *AuditService {
	return &AuditService{events: events}
}

var _ core.AuditUseCase = (*AuditService)(nil)

func (s *AuditService) List(
	ctx context.Context,
	filter domain.AuditListFilter,
	request authBaseDomain.PageRequest,
) (authBaseDomain.Page[domain.AuditEvent], error) {
	if s.events == nil {
		return authBaseDomain.Page[domain.AuditEvent]{}, fmt.Errorf("audit store is required")
	}
	cleaned, err := validateAuditFilter(filter)
	if err != nil {
		return authBaseDomain.Page[domain.AuditEvent]{}, err
	}
	page, err := s.events.List(ctx, cleaned, request)
	if err != nil {
		return authBaseDomain.Page[domain.AuditEvent]{}, fmt.Errorf("list audit events: %w", err)
	}
	return page, nil
}

func validateAuditFilter(filter domain.AuditListFilter) (domain.AuditListFilter, error) {
	if filter.Action != "" && !domain.KnownAuditAction(filter.Action) {
		return domain.AuditListFilter{}, fmt.Errorf("%w: action is not supported", domain.ErrInvalidAuditFilter)
	}
	if filter.Outcome != "" && !domain.KnownAuditOutcome(filter.Outcome) {
		return domain.AuditListFilter{}, fmt.Errorf("%w: outcome is not supported", domain.ErrInvalidAuditFilter)
	}
	if filter.ActorID != "" {
		filter.ActorID = domain.NormalizeAuditActor(filter.ActorID)
	}
	if !filter.From.IsZero() && !filter.To.IsZero() {
		if !filter.From.Before(filter.To) {
			return domain.AuditListFilter{}, fmt.Errorf("%w: from must be before to", domain.ErrInvalidAuditFilter)
		}
		if filter.To.Sub(filter.From) > maxAuditRange {
			return domain.AuditListFilter{}, fmt.Errorf("%w: time range exceeds 90 days", domain.ErrInvalidAuditFilter)
		}
	}
	return filter, nil
}
