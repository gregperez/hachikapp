package core

import (
	"context"

	"github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

type UserUseCase interface {
	core.GenericUseCase[domain.User]
	FindAll(ctx context.Context, request authBaseDomain.PageRequest) (authBaseDomain.Page[domain.User], error)
	FindByEmail(ctx context.Context, email string) (domain.User, error)
	Create(ctx context.Context, user domain.User) error
	FindById(ctx context.Context, id string) (domain.User, error)
	FindFullById(ctx context.Context, id string) (domain.User, error)
	UpdateOwnProfile(ctx context.Context, actorID, userID, name string) error
	AdminUpdateUser(ctx context.Context, userID, name, email string) error
	AssignRoles(ctx context.Context, actorID, userID string, roleIDs []string) error
	SetAdminFlag(ctx context.Context, actorID, userID string, isAdmin bool) error
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
	SetActive(ctx context.Context, actorID, userID string, active bool) error
}
