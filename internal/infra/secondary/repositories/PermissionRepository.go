package repositories

import (
	"jwt/base/internal/core/domain"
	"jwt/base/internal/core/ports"
	"jwt/base/internal/infra/secondary/repositories/dto"
	"sync"
)

var (
	permissionServiceSingleton    *ports.RepositoryPort
	permissionServiceInstanceOnce sync.Once
)

type PermissionRepository struct {
	permissionDao *IGenericDAO
}

func NewPermissionInstance(permissionDao *IGenericDAO) *ports.RepositoryPort {
	permissionServiceInstanceOnce.Do(func() {
		var service ports.RepositoryPort = &PermissionRepository{
			permissionDao: permissionDao,
		}
		permissionServiceSingleton = &service
	})
	return permissionServiceSingleton
}

func (repository *PermissionRepository) Save(v interface{}) error {
	return nil
}

func (repository *PermissionRepository) FindById(id string) (interface{}, error) {
	return nil, nil
}

func (r *PermissionRepository) FindAllByUser(userId string) ([]interface{}, error) {
	permissions, err := (*r.permissionDao).FindByUserId(userId)
	if err != nil {
		return nil, err
	}

	list := make([]interface{}, 0)

	for _, permission := range permissions {
		list = append(list, domain.Permission{
			Entity: (permission).(dto.PermissionDTO).Entity,
		})
	}

	return list, nil
}
