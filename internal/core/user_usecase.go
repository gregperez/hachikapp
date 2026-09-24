package core

import (
	"context"

	"github.com/WilsonSayago/authBase/v3/core"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
)

type UserUseCase interface {
	core.GenericUseCase[domain.User]
	FindAll(pageSize, offset int) ([]domain.User, int, error)
	FindByEmail(email string) (domain.User, error)
	Create(user domain.User) error
	FindById(id string) (domain.User, error)
	FindFullById(id string) (domain.User, error)
	UpdateOwnProfile(actorID, userID, name string) error
	AdminUpdateUser(userID, name, email string) error
	AssignRoles(actorID, userID string, roleIDs []string) error
	SetAdminFlag(actorID, userID string, isAdmin bool) error
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
	ChangeStatus(actorID, userID string) error
}
