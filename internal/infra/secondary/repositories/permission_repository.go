package repositories

import (
	"context"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/dao"
)

type PermissionRepository struct {
	permissionDao dao.IPermissionDao
}

func NewPermissionRepository(permissionDao dao.IPermissionDao) *PermissionRepository {
	return &PermissionRepository{permissionDao: permissionDao}
}

// NewPermissionInstance is an alias for NewPermissionRepository.
//
// Deprecated: use NewPermissionRepository.
func NewPermissionInstance(permissionDao dao.IPermissionDao) ports.PermissionPort {
	return NewPermissionRepository(permissionDao)
}

func (r *PermissionRepository) FindAllByUser(ctx context.Context, userId string) ([]authBaseDomain.Permission, error) {
	permissions, err := r.permissionDao.FindAllByUserId(ctx, userId)
	if err != nil {
		return nil, err
	}

	list := make([]authBaseDomain.Permission, 0, len(permissions))
	for _, permission := range permissions {
		list = append(list, authBaseDomain.Permission{
			Entity: permission.Entity,
			Create: permission.Create,
			Read:   permission.Read,
			Update: permission.Update,
			Delete: permission.Delete,
		})
	}
	return list, nil
}
