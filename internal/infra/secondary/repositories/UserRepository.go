package repositories

import (
	"fmt"
	authBaseDomain "github.com/WilsonSayago/authBase/core/domain"
	authBasePorts "github.com/WilsonSayago/authBase/core/port"
	"github.com/WilsonSayago/initModules"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/core/ports"
	"jwt/base/internal/infra/secondary/repositories/dto"
	"jwt/base/internal/infra/secondary/repositories/postgres/dao"
)

type UserRepository struct {
	userDao           dao.IUserDao
	roleDao           dao.IRoleDao
	validationService authBasePorts.ValidationPort
}

func NewUserInstance(userDao dao.IUserDao, roleDao dao.IRoleDao, validationService authBasePorts.ValidationPort) ports.UserPort {
	instance := initModules.GetInstance("UserRepository", func() interface{} {
		return &UserRepository{
			userDao:           userDao,
			roleDao:           roleDao,
			validationService: validationService,
		}
	})
	return instance.(*UserRepository)
}

func (repository *UserRepository) FindByEmail(email string) (domain.User, error) {
	userDTO, err := repository.userDao.FindByEmail(email)
	if err != nil {
		return domain.User{}, err
	}

	user := toUserDomain(userDTO)
	return user, nil
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

func (repository *UserRepository) Save(v domain.User) error {
	// find user by email
	userFound, err := (repository.userDao).FindByEmail(v.GetEmail())
	if err != nil {
		return err
	}
	if userFound.Id != "" {
		return fmt.Errorf("user already exists")
	}

	hashedPassword, err := (repository.validationService).HashPassword(v.GetPassword())
	if err != nil {
		return err
	}
	var userDTO = dto.UserDTO{
		Name:     v.GetName(),
		Email:    v.GetEmail(),
		Password: hashedPassword,
		IsAdmin:  v.GetIsAdmin(),
	}
	err = (repository.userDao).Save(userDTO)
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

func (repository *UserRepository) FindAll(pageSize, offset int) ([]domain.User, int, error) {
	totalRow, err := (repository.userDao).Count()
	users, err := (repository.userDao).FindAll(pageSize, offset)
	if err != nil {
		return []domain.User{}, 0, err
	}

	list := make([]domain.User, 0)
	for _, user := range users {
		list = append(list, toUserDomain(user))
	}
	return list, totalRow, nil
}

func (repository *UserRepository) Update(v domain.User) error {
	userDTO := dto.UserDTO{
		BaseDTO: dto.BaseDTO{Id: v.GetId()},
		Name:    v.GetName(),
		Email:   v.GetEmail(),
		IsAdmin: v.GetIsAdmin(),
	}
	err := (repository.userDao).Update(userDTO)
	if err != nil {
		return err
	}
	return nil
}

func (repository *UserRepository) ChangeStatus(id string) error {
	err := (repository.userDao).ChangeStatus(id)
	if err != nil {
		return err
	}
	return nil
}

func toUserDomain(user dto.UserDTO) domain.User {
	var roles []authBaseDomain.Role
	for _, role := range user.Roles {
		roles = append(roles, ToRoleDomain(role))
	}
	return domain.User{
		UserGeneric: authBaseDomain.NewUserGeneric(
			user.Id,
			user.Name,
			user.Email,
			user.Password,
			roles,
			user.IsAdmin,
			user.Active,
		),
	}
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
