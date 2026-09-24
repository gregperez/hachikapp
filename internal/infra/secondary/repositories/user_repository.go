package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/dto"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/dao"

	authBaseCore "github.com/WilsonSayago/authBase/v3/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v3/core/domain"
	authBasePorts "github.com/WilsonSayago/authBase/v3/core/port"
	initModules "github.com/WilsonSayago/initModules/v2"
	log "github.com/sirupsen/logrus"
)

type UserRepository struct {
	userDao           dao.IUserDao
	roleDao           dao.IRoleDao
	userRoleDao       dao.IUserRoleDao
	validationService authBasePorts.ValidationPort
}

func NewUserInstance(userDao dao.IUserDao, roleDao dao.IRoleDao, userRoleDao dao.IUserRoleDao, validationService authBasePorts.ValidationPort) ports.UserPort {
	return initModules.Once(func() *UserRepository {
		return &UserRepository{
			userDao:           userDao,
			roleDao:           roleDao,
			userRoleDao:       userRoleDao,
			validationService: validationService,
		}
	})
}

// FindByID implements authBase UserReader (identity without password).
func (repository *UserRepository) FindByID(ctx context.Context, id string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	user, err := repository.FindFullById(id)
	if err != nil {
		return domain.User{}, authBaseCore.ErrNotFound
	}
	return user, nil
}

// FindCredentialsByUsername implements authBase CredentialReader.
func (repository *UserRepository) FindCredentialsByUsername(ctx context.Context, username string) (authBasePorts.CredentialRecord, error) {
	if err := ctx.Err(); err != nil {
		return authBasePorts.CredentialRecord{}, err
	}
	rec, err := repository.userDao.FindCredentialsByEmail(username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return authBasePorts.CredentialRecord{}, authBaseCore.ErrNotFound
		}
		return authBasePorts.CredentialRecord{}, err
	}
	return authBasePorts.CredentialRecord{
		UserID:       rec.Id,
		Username:     rec.Email,
		PasswordHash: rec.Password,
		Active:       rec.Active,
	}, nil
}

func (repository *UserRepository) FindByEmail(email string) (domain.User, error) {
	userDTO, err := repository.userDao.FindByEmail(email)
	if err != nil {
		return domain.User{}, fmt.Errorf("errors.users.not-found")
	}
	if userDTO.Id == "" {
		return domain.User{}, nil
	}
	return toUserDomain(userDTO), nil
}

func (repository *UserRepository) FindFullById(id string) (domain.User, error) {
	user, err := repository.userDao.FindById(id)
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
	userFound, err := repository.userDao.FindByEmail(user.GetEmail())
	if err != nil {
		return fmt.Errorf("errors.generals.errors")
	}
	if userFound.Id != "" {
		return fmt.Errorf("errors.users.already-exists")
	}

	plain := user.PlainPassword()
	if plain == "" {
		return fmt.Errorf("errors.users.hash-password")
	}
	hashedPassword, err := repository.validationService.HashPassword(plain)
	if err != nil {
		return fmt.Errorf("errors.users.hash-password")
	}

	tx, err := repository.userDao.BeginTx()
	if err != nil {
		log.Errorf("Error al iniciar transacción: %v", err)
		return fmt.Errorf("errors.transaction.begin")
	}

	var txError error
	defer func() {
		if txError != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Errorf("Error al hacer rollback: %v", rbErr)
			}
		}
	}()

	userDTO := dto.UserDTO{
		Name:     user.GetName(),
		Email:    user.GetEmail(),
		Password: hashedPassword,
		IsAdmin:  user.GetIsAdmin(),
	}

	txError = repository.userDao.SaveWithTx(tx, &userDTO)
	if txError != nil {
		return fmt.Errorf("errors.users.save")
	}

	for _, role := range user.GetRole() {
		txError = repository.userRoleDao.SaveWithTx(tx, userDTO.Id, role.Id)
		if txError != nil {
			return fmt.Errorf("errors.users.save-user-role")
		}
	}

	txError = tx.Commit()
	if txError != nil {
		log.Errorf("Error al hacer commit: %v", txError)
		return fmt.Errorf("errors.transaction.commit")
	}
	return nil
}

func (repository *UserRepository) FindById(id string) (domain.User, error) {
	return repository.FindFullById(id)
}

func (repository *UserRepository) FindAll(pageSize, offset int) ([]domain.User, int, error) {
	totalRow, err := repository.userDao.Count()
	if err != nil {
		return nil, 0, fmt.Errorf("errors.users.get-all")
	}
	users, err := repository.userDao.FindAll(pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("errors.users.get-all")
	}
	list := make([]domain.User, 0, len(users))
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

	tx, err := repository.userDao.BeginTx()
	if err != nil {
		log.Errorf("Error al iniciar transacción: %v", err)
		return fmt.Errorf("errors.transaction.begin")
	}

	var txError error
	defer func() {
		if txError != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Errorf("Error al hacer rollback: %v", rbErr)
			}
		}
	}()

	txError = repository.userDao.UpdateWithTx(tx, userDTO)
	if txError != nil {
		return fmt.Errorf("errors.users.update")
	}

	txError = repository.userRoleDao.DeleteAllByUserIdWithTx(tx, userDTO.Id)
	if txError != nil {
		return fmt.Errorf("errors.users.delete-user-role")
	}

	for _, role := range user.GetRole() {
		txError = repository.userRoleDao.SaveWithTx(tx, userDTO.Id, role.Id)
		if txError != nil {
			return fmt.Errorf("errors.users.save-user-role")
		}
	}

	txError = tx.Commit()
	if txError != nil {
		log.Errorf("Error al hacer commit: %v", txError)
		return fmt.Errorf("errors.transaction.commit")
	}
	return nil
}

func (repository *UserRepository) ChangeStatus(id string) error {
	tx, err := repository.userDao.BeginTx()
	if err != nil {
		log.Errorf("Error al iniciar transacción: %v", err)
		return fmt.Errorf("errors.transaction.begin")
	}

	var txError error
	defer func() {
		if txError != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Errorf("Error al hacer rollback: %v", rbErr)
			}
		}
	}()

	txError = repository.userDao.ChangeStatusWithTx(tx, id)
	if txError != nil {
		return fmt.Errorf("errors.users.change-status")
	}

	txError = tx.Commit()
	if txError != nil {
		log.Errorf("Error al hacer commit: %v", txError)
		return fmt.Errorf("errors.transaction.commit")
	}
	return nil
}

func toUserDomain(user dto.UserDTO) domain.User {
	roles := make([]authBaseDomain.Role, 0, len(user.Roles))
	for _, role := range user.Roles {
		roles = append(roles, ToRoleDomain(role))
	}
	return domain.User{
		UserGeneric: authBaseDomain.NewUserGeneric(
			user.Id,
			user.Name,
			user.Email,
			roles,
			user.IsAdmin,
			user.Active,
		),
	}
}

func ToPermissionDTO(permissions []authBaseDomain.Permission) []dto.PermissionDTO {
	permissionDTO := make([]dto.PermissionDTO, 0, len(permissions))
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
	permission := make([]authBaseDomain.Permission, 0, len(permissionsDto))
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
