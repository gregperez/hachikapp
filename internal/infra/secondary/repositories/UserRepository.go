package repositories

import (
	"github.com/google/uuid"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/core/ports"
	"jwt/base/internal/infra/secondary/repositories/dto"
	"sync"
)

var (
	serviceSingleton    *ports.RepositoryPort
	serviceInstanceOnce sync.Once
)

type UserRepository struct {
	userDao *IGenericDAO
}

func NewUserInstance(userDao *IGenericDAO) *ports.RepositoryPort {
	serviceInstanceOnce.Do(func() {
		var service ports.RepositoryPort = &UserRepository{
			userDao: userDao,
		}
		serviceSingleton = &service
	})
	return serviceSingleton
}

func (repository *UserRepository) Save(v interface{}) error {
	var userDTO = dto.UserDTO{
		Id:          uuid.New(),
		Email:       v.(domain.User).Email,
		Password:    v.(domain.User).Password,
		Permissions: ToPermissionDTO(v.(domain.User).Permissions),
		IsAdmin:     v.(domain.User).IsAdmin,
	}
	err := (*repository.userDao).Save(userDTO)
	if err != nil {
		return err
	}
	return nil
}

func ToPermissionDTO(permissions []domain.Permission) []dto.PermissionDTO {
	var permissionDTO []dto.PermissionDTO
	for _, permission := range permissions {
		permissionDTO = append(permissionDTO, dto.PermissionDTO{
			Entity: permission.Entity,
			Create: permission.Create,
			Read:   permission.Read,
			Update: permission.Update,
			Delete: permission.Delete,
		})
	}
	return permissionDTO
}
