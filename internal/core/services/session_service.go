package services

import (
	"context"
	"fmt"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
)

type SessionService struct {
	sessions ports.SessionPort
}

func NewSessionService(sessions ports.SessionPort) *SessionService {
	return &SessionService{sessions: sessions}
}

var _ core.SessionUseCase = (*SessionService)(nil)

func (s *SessionService) ListMine(
	ctx context.Context,
	actorID, currentFamilyID string,
	request authBaseDomain.PageRequest,
) (authBaseDomain.Page[domain.Session], error) {
	if actorID == "" {
		return authBaseDomain.Page[domain.Session]{}, fmt.Errorf("actor id is required")
	}
	page, err := s.sessions.ListActiveByUser(ctx, actorID, request)
	if err != nil {
		return authBaseDomain.Page[domain.Session]{}, fmt.Errorf("list sessions: %w", err)
	}
	if currentFamilyID == "" {
		return page, nil
	}
	for i := range page.Items {
		page.Items[i].Current = page.Items[i].FamilyID == currentFamilyID
	}
	return page, nil
}

func (s *SessionService) RevokeMine(ctx context.Context, actorID, familyID, currentFamilyID string) (bool, error) {
	if actorID == "" || familyID == "" {
		return false, domain.ErrNotFound
	}
	if err := s.sessions.RevokeFamilyForUser(ctx, actorID, familyID); err != nil {
		return false, fmt.Errorf("revoke session: %w", err)
	}
	return currentFamilyID != "" && familyID == currentFamilyID, nil
}

func (s *SessionService) RevokeAllMine(ctx context.Context, actorID string) error {
	if actorID == "" {
		return fmt.Errorf("actor id is required")
	}
	if err := s.sessions.RevokeUser(ctx, actorID); err != nil {
		return fmt.Errorf("revoke all sessions: %w", err)
	}
	return nil
}
