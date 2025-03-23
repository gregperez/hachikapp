package repositories

import (
	"fmt"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/core/ports"
	"jwt/base/internal/infra/secondary/repositories/dto"
	"jwt/base/internal/infra/secondary/repositories/postgres/dao"

	authBaseDomain "github.com/WilsonSayago/authBase/core/domain"
	authBasePorts "github.com/WilsonSayago/authBase/core/port"
	"github.com/WilsonSayago/initModules"
	log "github.com/sirupsen/logrus"
)

type UserRepository struct {
	userDao           dao.IUserDao
	roleDao           dao.IRoleDao
	userRoleDao       dao.IUserRoleDao
	validationService authBasePorts.ValidationPort
}

func NewUserInstance(userDao dao.IUserDao, roleDao dao.IRoleDao, userRoleDao dao.IUserRoleDao, validationService authBasePorts.ValidationPort) ports.UserPort {
	instance := initModules.GetInstance("UserRepository", func() interface{} {
		return &UserRepository{
			userDao:           userDao,
			roleDao:           roleDao,
			userRoleDao:       userRoleDao,
			validationService: validationService,
		}
	})
	return instance.(*UserRepository)
}

func (repository *UserRepository) FindByEmail(email string) (domain.User, error) {
	userDTO, err := repository.userDao.FindByEmail(email)
	if err != nil {
		return domain.User{}, fmt.Errorf("errors.users.not-found")
	}

	user := toUserDomain(userDTO)
	return user, nil
}

func (repository *UserRepository) FindFullById(id string) (domain.User, error) {
	user, err := (repository.userDao).FindById(id)
	if err != nil {
		return domain.User{}, fmt.Errorf("errors.users.not-found")
	}

	user.Roles, err = repository.roleDao.FindAllByUserId(user.Id)

	if err != nil {
		return domain.User{}, fmt.Errorf("errors.roles.not-found")
	}

	return toUserDomain(user), nil
}

func (repository *UserRepository) Save(user domain.User) error {
	userFound, err := (repository.userDao).FindByEmail(user.GetEmail())
	if err != nil {
		return fmt.Errorf("errors.generals.errors")
	}
	if userFound.Id != "" {
		return fmt.Errorf("errors.users.already-exists")
	}

	hashedPassword, err := (repository.validationService).HashPassword(user.GetPassword())
	if err != nil {
		return fmt.Errorf("errors.users.hash-password")
	}

	// Iniciar transacción
	tx, err := repository.userDao.BeginTx()
	if err != nil {
		log.Errorf("Error al iniciar transacción: %v", err)
		return fmt.Errorf("errors.transaction.begin")
	}

	// Si hay un error en cualquier punto, hacer rollback
	var txError error
	defer func() {
		if txError != nil {
			err := tx.Rollback()
			if err != nil {
				log.Errorf("Error al hacer rollback: %v", err)
			}
		}
	}()

	var userDTO = dto.UserDTO{
		Name:     user.GetName(),
		Email:    user.GetEmail(),
		Password: hashedPassword,
		IsAdmin:  user.GetIsAdmin(),
	}

	// Guardar usuario con transacción
	txError = repository.userDao.SaveWithTx(tx, &userDTO)
	if txError != nil {
		return fmt.Errorf("errors.users.save")
	}

	// Guardar roles de usuario con transacción
	for _, role := range user.GetRole() {
		txError = repository.userRoleDao.SaveWithTx(tx, userDTO.Id, role.Id)
		if txError != nil {
			return fmt.Errorf("errors.users.save-user-role")
		}
	}

	// Commit de la transacción
	txError = tx.Commit()
	if txError != nil {
		log.Errorf("Error al hacer commit: %v", txError)
		return fmt.Errorf("errors.transaction.commit")
	}

	return nil
}

func (repository *UserRepository) FindById(id string) (domain.User, error) {
	user, err := (repository.userDao).FindById(id)
	if err != nil {
		return domain.User{}, fmt.Errorf("errors.users.not-found")
	}

	user.Roles, err = repository.roleDao.FindAllByUserId(user.Id)

	if err != nil {
		return domain.User{}, fmt.Errorf("errors.roles.not-found")
	}

	response := toUserDomain(user)
	return response, nil
}

func (repository *UserRepository) FindAll(pageSize, offset int) ([]domain.User, int, error) {
	totalRow, err := (repository.userDao).Count()
	users, err := (repository.userDao).FindAll(pageSize, offset)
	if err != nil {
		return []domain.User{}, 0, fmt.Errorf("errors.users.get-all")
	}

	list := make([]domain.User, 0)
	for _, user := range users {
		list = append(list, toUserDomain(user))
	}
	return list, totalRow, nil
}

func (repository *UserRepository) Update(user domain.User) error {
	userDTO := dto.UserDTO{
		BaseDTO: dto.BaseDTO{Id: user.GetId()},
		Name:    user.GetName(),
		Email:   user.GetEmail(),
		IsAdmin: user.GetIsAdmin(),
	}

	// Iniciar transacción
	tx, err := repository.userDao.BeginTx()
	if err != nil {
		log.Errorf("Error al iniciar transacción: %v", err)
		return fmt.Errorf("errors.transaction.begin")
	}

	// Si hay un error en cualquier punto, hacer rollback
	var txError error
	defer func() {
		if txError != nil {
			err := tx.Rollback()
			if err != nil {
				log.Errorf("Error al hacer rollback: %v", err)
			}
		}
	}()

	// Actualizar usuario con transacción
	txError = repository.userDao.UpdateWithTx(tx, userDTO)
	if txError != nil {
		return fmt.Errorf("errors.users.update")
	}

	// Eliminar todos los roles del usuario con transacción
	txError = repository.userRoleDao.DeleteAllByUserIdWithTx(tx, userDTO.Id)
	if txError != nil {
		return fmt.Errorf("errors.users.delete-user-role")
	}

	// Agregar los nuevos roles con transacción
	for _, role := range user.GetRole() {
		txError = repository.userRoleDao.SaveWithTx(tx, userDTO.Id, role.Id)
		if txError != nil {
			return fmt.Errorf("errors.users.save-user-role")
		}
	}

	// Commit de la transacción
	txError = tx.Commit()
	if txError != nil {
		log.Errorf("Error al hacer commit: %v", txError)
		return fmt.Errorf("errors.transaction.commit")
	}

	return nil
}

func (repository *UserRepository) ChangeStatus(id string) error {
	// Iniciar transacción
	tx, err := repository.userDao.BeginTx()
	if err != nil {
		log.Errorf("Error al iniciar transacción: %v", err)
		return fmt.Errorf("errors.transaction.begin")
	}

	// Si hay un error, hacer rollback
	var txError error
	defer func() {
		if txError != nil {
			err := tx.Rollback()
			if err != nil {
				log.Errorf("Error al hacer rollback: %v", err)
			}
		}
	}()

	// Cambiar estado con transacción
	txError = repository.userDao.ChangeStatusWithTx(tx, id)
	if txError != nil {
		return fmt.Errorf("errors.users.change-status")
	}

	// Commit de la transacción
	txError = tx.Commit()
	if txError != nil {
		log.Errorf("Error al hacer commit: %v", txError)
		return fmt.Errorf("errors.transaction.commit")
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
