package repositories

import (
	"github.com/WilsonSayago/initModules"
	middlewareDomain "github.com/WilsonSayago/middleware/core/domains"
	"jwt/base/internal/core/ports"
	"jwt/base/internal/infra/secondary/repositories/postgres/dao"
	//"sync"
)

type PermissionRepository struct {
	permissionDao dao.IPermissionDao
}

func NewPermissionInstance(permissionDao dao.IPermissionDao) ports.PermissionPort {
	instance := initModules.GetInstance("PermissionRepository", func() interface{} {
		return &PermissionRepository{
			permissionDao: permissionDao,
		}
	})
	return instance.(*PermissionRepository)
}

func (r *PermissionRepository) FindAllByUser(userId string) ([]middlewareDomain.Permission, error) {
	permissions, err := (r.permissionDao).FindAllByUserId(userId)
	if err != nil {
		return nil, err
	}

	list := make([]middlewareDomain.Permission, 0)

	for _, permission := range permissions {
		list = append(list, middlewareDomain.Permission{
			Entity: permission.Entity,
			Create: permission.Create,
			Read:   permission.Read,
			Update: permission.Update,
			Delete: permission.Delete,
		})
	}

	return list, nil
}
