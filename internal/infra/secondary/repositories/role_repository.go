package repositories

import (
	"context"
	"errors"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/dto"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/dao"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	authBasePorts "github.com/WilsonSayago/authBase/v4/core/port"
)

type RoleRepository struct {
	roleDao dao.IRoleDao
}

func NewRoleRepository(roleDao dao.IRoleDao) *RoleRepository {
	return &RoleRepository{roleDao: roleDao}
}

// NewRoleInstance is an alias for NewRoleRepository.
//
// Deprecated: use NewRoleRepository.
func NewRoleInstance(roleDao dao.IRoleDao) authBasePorts.RolePort {
	return NewRoleRepository(roleDao)
}

func (r RoleRepository) FindById(ctx context.Context, id string) (authBaseDomain.Role, error) {
	role, err := r.roleDao.FindById(ctx, id)
	if err != nil {
		return authBaseDomain.Role{}, err
	}
	return ToRoleDomain(role), nil
}

func (r RoleRepository) FindAll(
	ctx context.Context,
	request authBaseDomain.PageRequest,
) (authBaseDomain.Page[authBaseDomain.Role], error) {
	roles, hasNext, err := r.roleDao.FindAll(ctx, request)
	if err != nil {
		return authBaseDomain.Page[authBaseDomain.Role]{}, err
	}
	domainRoles := make([]authBaseDomain.Role, 0, len(roles))
	for _, role := range roles {
		domainRoles = append(domainRoles, ToRoleDomain(role))
	}
	return authBaseDomain.Page[authBaseDomain.Role]{Items: domainRoles, HasNext: hasNext}, nil
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
			Id:        role.Id,
			CreatedAt: role.CreatedAt,
			UpdatedAt: role.UpdatedAt,
			Active:    role.Active,
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
