package repositories

import (
	"context"
	"errors"

	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/dto"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/dao"

	authBaseDomain "github.com/WilsonSayago/authBase/v3/core/domain"
	authBasePorts "github.com/WilsonSayago/authBase/v3/core/port"
	initModules "github.com/WilsonSayago/initModules/v2"
)

type RoleRepository struct {
	roleDao dao.IRoleDao
}

func NewRoleInstance(roleDao dao.IRoleDao) authBasePorts.RolePort {
	return initModules.Once(func() *RoleRepository {
		return &RoleRepository{roleDao: roleDao}
	})
}

func (r RoleRepository) FindById(ctx context.Context, id string) (authBaseDomain.Role, error) {
	role, err := r.roleDao.FindById(ctx, id)
	if err != nil {
		return authBaseDomain.Role{}, err
	}
	return ToRoleDomain(role), nil
}

func (r RoleRepository) FindAll(ctx context.Context, pageSize, offset int) ([]authBaseDomain.Role, int, error) {
	total, err := r.roleDao.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	roles, err := r.roleDao.FindAll(ctx, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	domainRoles := make([]authBaseDomain.Role, 0, len(roles))
	for _, role := range roles {
		domainRoles = append(domainRoles, ToRoleDomain(role))
	}
	return domainRoles, total, nil
}

func (r RoleRepository) Save(ctx context.Context, role authBaseDomain.Role) (authBaseDomain.Role, error) {
	roleFound, err := r.roleDao.FindByName(ctx, role.Name)
	if err != nil {
		return authBaseDomain.Role{}, err
	}
	if roleFound.Id != "" {
		return authBaseDomain.Role{}, domain.ErrConflict
	}
	saved, err := r.roleDao.Save(ctx, ToRoleDTO(role))
	if err != nil {
		return authBaseDomain.Role{}, err
	}
	return ToRoleDomain(saved), nil
}

func (r RoleRepository) Update(ctx context.Context, role authBaseDomain.Role) error {
	_, err := r.roleDao.FindById(ctx, role.Id)
	if err != nil {
		return err
	}
	return r.roleDao.Update(ctx, ToRoleDTO(role))
}

func (r RoleRepository) SetActive(ctx context.Context, id string, active bool) error {
	err := r.roleDao.SetActive(ctx, id, active)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ErrNotFound
	}
	return err
}

func ToRoleDomain(role dto.RoleDTO) authBaseDomain.Role {
	return authBaseDomain.Role{
		Base: authBaseDomain.Base{
			Id:     role.Id,
			Active: role.Active,
		},
		Name:        role.Name,
		Routes:      role.Routes,
		Permissions: ToPermission(role.Permissions),
	}
}

func ToRoleDTO(role authBaseDomain.Role) dto.RoleDTO {
	return dto.RoleDTO{
		BaseDTO: dto.BaseDTO{
			Id: role.Id,
		},
		Name:        role.Name,
		Routes:      role.Routes,
		Permissions: ToPermissionDTO(role.Permissions),
	}
}
