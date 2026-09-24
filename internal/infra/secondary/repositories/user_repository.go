package repositories

import (
	"context"
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

// NewUserRepositoryForTest builds a repository without process-wide Once (tests only).
func NewUserRepositoryForTest(userDao dao.IUserDao, roleDao dao.IRoleDao, userRoleDao dao.IUserRoleDao, validationService authBasePorts.ValidationPort) *UserRepository {
	return &UserRepository{
		userDao:           userDao,
		roleDao:           roleDao,
		userRoleDao:       userRoleDao,
		validationService: validationService,
	}
}

func (repository *UserRepository) FindByID(ctx context.Context, id string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	user, err := repository.FindFullById(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.User{}, authBaseCore.ErrNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

func (repository *UserRepository) FindCredentialsByUsername(ctx context.Context, username string) (authBasePorts.CredentialRecord, error) {
	if err := ctx.Err(); err != nil {
		return authBasePorts.CredentialRecord{}, err
	}
	rec, err := repository.userDao.FindCredentialsByEmail(ctx, username)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
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

func (repository *UserRepository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	userDTO, err := repository.userDao.FindByEmail(ctx, email)
	if err != nil {
		return domain.User{}, fmt.Errorf("find user by email: %w", err)
	}
	if userDTO.Id == "" {
		return domain.User{}, nil
	}
	return toUserDomain(userDTO), nil
}

func (repository *UserRepository) FindFullById(ctx context.Context, id string) (domain.User, error) {
	user, err := repository.userDao.FindById(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	user.Roles, err = repository.roleDao.FindAllByUserId(ctx, user.Id)
	if err != nil {
		return domain.User{}, fmt.Errorf("load user roles: %w", err)
	}
	return toUserDomain(user), nil
}

func (repository *UserRepository) Save(ctx context.Context, user domain.User) error {
	userFound, err := repository.userDao.FindByEmail(ctx, user.GetEmail())
	if err != nil {
		return fmt.Errorf("check user exists: %w", err)
	}
	if userFound.Id != "" {
		return domain.ErrConflict
	}

	plain := user.PlainPassword()
	if plain == "" {
		return fmt.Errorf("errors.users.hash-password")
	}
	hashedPassword, err := repository.validationService.HashPassword(plain)
	if err != nil {
		return fmt.Errorf("errors.users.hash-password")
	}

	tx, err := repository.userDao.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin user save tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	userDTO := dto.UserDTO{
		Name:     user.GetName(),
		Email:    user.GetEmail(),
		Password: hashedPassword,
		IsAdmin:  user.GetIsAdmin(),
	}
	if err := repository.userDao.SaveWithTx(ctx, tx, &userDTO); err != nil {
		return err
	}
	for _, role := range user.GetRole() {
		if err := repository.userRoleDao.SaveWithTx(ctx, tx, userDTO.Id, role.Id); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit user save: %w", err)
	}
	return nil
}

// SaveFirstAdmin creates the first administrator under a transaction advisory lock.
func (repository *UserRepository) SaveFirstAdmin(ctx context.Context, user domain.User) error {
	plain := user.PlainPassword()
	if plain == "" {
		return fmt.Errorf("errors.users.hash-password")
	}
	hashedPassword, err := repository.validationService.HashPassword(plain)
	if err != nil {
		return fmt.Errorf("errors.users.hash-password")
	}

	tx, err := repository.userDao.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin first-admin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Serialize concurrent bootstraps across connections.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(87420010)`); err != nil {
		return fmt.Errorf("lock first-admin: %w", err)
	}
	n, err := repository.userDao.CountAdminsWithTx(ctx, tx)
	if err != nil {
		return err
	}
	if n > 0 {
		return domain.ErrAdminExists
	}

	userFound, err := repository.userDao.FindByEmail(ctx, user.GetEmail())
	if err != nil {
		return fmt.Errorf("check user exists: %w", err)
	}
	if userFound.Id != "" {
		return domain.ErrConflict
	}

	userDTO := dto.UserDTO{
		Name:     user.GetName(),
		Email:    user.GetEmail(),
		Password: hashedPassword,
		IsAdmin:  true,
	}
	if err := repository.userDao.SaveWithTx(ctx, tx, &userDTO); err != nil {
		return err
	}
	for _, role := range user.GetRole() {
		if err := repository.userRoleDao.SaveWithTx(ctx, tx, userDTO.Id, role.Id); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit first-admin: %w", err)
	}
	return nil
}

func (repository *UserRepository) FindById(ctx context.Context, id string) (domain.User, error) {
	return repository.FindFullById(ctx, id)
}

func (repository *UserRepository) FindAll(ctx context.Context, pageSize, offset int) ([]domain.User, int, error) {
	totalRow, err := repository.userDao.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	users, err := repository.userDao.FindAll(ctx, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	list := make([]domain.User, 0, len(users))
	for _, user := range users {
		list = append(list, toUserDomain(user))
	}
	return list, totalRow, nil
}

func (repository *UserRepository) Update(ctx context.Context, user domain.User) error {
	tx, err := repository.userDao.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin user update tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	userDTO := dto.UserDTO{
		BaseDTO: dto.BaseDTO{Id: user.GetId()},
		Name:    user.GetName(),
		Email:   user.GetEmail(),
		IsAdmin: user.GetIsAdmin(),
	}
	if err := repository.userDao.UpdateWithTx(ctx, tx, userDTO); err != nil {
		return err
	}
	if err := repository.userRoleDao.DeleteAllByUserIdWithTx(ctx, tx, userDTO.Id); err != nil {
		return err
	}
	for _, role := range user.GetRole() {
		if err := repository.userRoleDao.SaveWithTx(ctx, tx, userDTO.Id, role.Id); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit user update: %w", err)
	}
	return nil
}

func (repository *UserRepository) SetActive(ctx context.Context, id string, active bool) error {
	return repository.userDao.SetActive(ctx, id, active)
}

func (repository *UserRepository) DemoteAdminIfNotLast(ctx context.Context, id string) error {
	return repository.userDao.DemoteAdminIfNotLast(ctx, id)
}

func (repository *UserRepository) DeactivateAdminIfNotLast(ctx context.Context, id string) error {
	return repository.userDao.DeactivateAdminIfNotLast(ctx, id)
}

func (repository *UserRepository) CountAdmins(ctx context.Context) (int, error) {
	return repository.userDao.CountAdmins(ctx)
}

func (repository *UserRepository) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	if id == "" || passwordHash == "" {
		return fmt.Errorf("errors.users.update-password")
	}
	return repository.userDao.UpdatePassword(ctx, id, passwordHash)
}

func (repository *UserRepository) FindRoleIDByName(ctx context.Context, name string) (string, error) {
	role, err := repository.roleDao.FindByName(ctx, name)
	if err != nil {
		return "", err
	}
	if role.Id == "" {
		return "", domain.ErrNotFound
	}
	return role.Id, nil
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
