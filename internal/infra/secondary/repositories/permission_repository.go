package repositories

import (
	"context"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/dao"
	"github.com/WilsonSayago/initModules/v2"
)

type PermissionRepository struct {
	permissionDao dao.IPermissionDao
}

func NewPermissionInstance(permissionDao dao.IPermissionDao) ports.PermissionPort {
	return initModules.Once(func() *PermissionRepository {
		return &PermissionRepository{permissionDao: permissionDao}
	})
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
