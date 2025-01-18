package repositories

import (
	authBaseDomain "github.com/WilsonSayago/authBase/core/domains"
	"github.com/WilsonSayago/initModules"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/core/ports"
	"jwt/base/internal/infra/secondary/repositories/dto"
	"jwt/base/internal/infra/secondary/repositories/postgres/dao"
)

type UserRepository struct {
	userDao       dao.IUserDao
	roleDao       dao.IRoleDao
	permissionDao dao.IPermissionDao
}

func (repository *UserRepository) FindByEmail(email string) (domain.User, error) {
	userDTO, err := repository.userDao.FindByEmail(email)
	if err != nil {
		return domain.User{}, err
	}
	
	user := toUserDomain(userDTO)
	return user, nil
}

func NewUserInstance(userDao dao.IUserDao, roleDao dao.IRoleDao) ports.UserPort {
	instance := initModules.GetInstance("UserRepository", func() interface{} {
		return &UserRepository{
			userDao: userDao,
			roleDao: roleDao,
		}
	})
	return instance.(*UserRepository)
}

func (repository *UserRepository) FindFullById(id string) (domain.User, error) {
	user, err := (repository.userDao).FindById(id)
	if err != nil {
		return domain.User{}, err
	}
	
	user.Roles, err = repository.roleDao.FindAllByUserId(user.Id)
	
	if err != nil {
		return domain.User{}, err
	}
	
	return toUserDomain(user), nil
}

func (repository *UserRepository) Save(v interface{}) error {
	var userDTO = dto.UserDTO{
		Email:       (v.(*domain.User)).GetEmail(),
		Password:    v.(*domain.User).GetPassword(),
		Permissions: ToPermissionDTO(v.(*domain.User).GetPermissions()),
		IsAdmin:     v.(*domain.User).GetIsAdmin(),
	}
	err := (repository.userDao).Save(userDTO)
	if err != nil {
		return err
	}
	return nil
}

func (repository *UserRepository) FindById(id string) (domain.User, error) {
	user, err := (repository.userDao).FindById(id)
	if err != nil {
		return domain.User{}, err
	}
	
	return toUserDomain(user), nil
}

func (repository *UserRepository) FindAll() ([]domain.User, error) {
	users, err := (repository.userDao).FindAll()
	if err != nil {
		return []domain.User{}, err
	}
	
	list := make([]domain.User, 0)
	for _, user := range users {
		list = append(list, toUserDomain(user))
	}
	return list, nil
}

func toUserDomain(user dto.UserDTO) domain.User {
	return domain.User{
		UserGeneric: authBaseDomain.NewUserGeneric(
			user.Id,
			user.Email,
			user.Password,
			ToRole(user.Roles),
			user.IsAdmin),
	}
}

func ToRole(roles []dto.RoleDTO) []authBaseDomain.Role {
	var role []authBaseDomain.Role
	for _, roleDto := range roles {
		role = append(role, authBaseDomain.Role{
			Base: authBaseDomain.Base{
				Id: roleDto.Id,
			},
			Name:        roleDto.Name,
			Permissions: ToPermission(roleDto.Permissions),
		})
	}
	return role
}

func ToPermissionDTO(permissions []authBaseDomain.Permission) []dto.PermissionDTO {
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

func ToPermission(permissionsDto []dto.PermissionDTO) []authBaseDomain.Permission {
	var permission []authBaseDomain.Permission
	for _, permissionDto := range permissionsDto {
		permission = append(permission, authBaseDomain.Permission{
			Entity: permissionDto.Entity,
			Create: permissionDto.Create,
			Read:   permissionDto.Read,
			Update: permissionDto.Update,
			Delete: permissionDto.Delete,
		})
	}
	return permission
}
