package repositories

import (
	authBaseDomain "github.com/WilsonSayago/authBase/v3/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/dao"
	"github.com/WilsonSayago/initModules/v2"
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

func (r *PermissionRepository) FindAllByUser(userId string) ([]authBaseDomain.Permission, error) {
	permissions, err := (r.permissionDao).FindAllByUserId(userId)
	if err != nil {
		return nil, err
	}

	list := make([]authBaseDomain.Permission, 0)

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
