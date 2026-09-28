package ports

import (
	"context"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
)

// SessionPort lists and revokes the caller's own refresh families.
type SessionPort interface {
	ListActiveByUser(ctx context.Context, userID string, request authBaseDomain.PageRequest) (authBaseDomain.Page[domain.Session], error)
	RevokeFamilyForUser(ctx context.Context, userID, familyID string) error
	RevokeUser(ctx context.Context, userID string) error
}
