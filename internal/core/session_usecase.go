package core

import (
	"context"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// SessionUseCase exposes self-service refresh-family management.
// The actor ID always comes from a validated access token, never from the body.
type SessionUseCase interface {
	ListMine(ctx context.Context, actorID, currentFamilyID string, request authBaseDomain.PageRequest) (authBaseDomain.Page[domain.Session], error)
	RevokeMine(ctx context.Context, actorID, familyID, currentFamilyID string) (revokedCurrent bool, err error)
	RevokeAllMine(ctx context.Context, actorID string) error
}
