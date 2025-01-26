package repositories

import (
	"fmt"
	"jwt/base/internal/infra/secondary/repositories/dto"
	"jwt/base/internal/infra/secondary/repositories/postgres/dao"

	"github.com/WilsonSayago/authBase/core/domain"
	authBasePorts "github.com/WilsonSayago/authBase/core/port"
	"github.com/WilsonSayago/initModules"
)

type RoleRepository struct {
	roleDao dao.IRoleDao
}

func NewRoleInstance(roleDao dao.IRoleDao) authBasePorts.RolePort {
	instance := initModules.GetInstance("RoleRepository", func() interface{} {
		return &RoleRepository{
			roleDao: roleDao,
		}
	})
	return instance.(*RoleRepository)
}

func (r RoleRepository) FindById(id string) (domain.Role, error) {
	role, err := (r.roleDao).FindById(id)
	if err != nil {
		return domain.Role{}, err
	}

	return ToRoleDomain(role), nil
}

func (r RoleRepository) FindAll() ([]domain.Role, error) {
	// find all roles
	roles, err := (r.roleDao).FindAll()
	if err != nil {
		return nil, err
	}

	// convert roles to domain
	var domainRoles []domain.Role
	for _, role := range roles {
		domainRoles = append(domainRoles, ToRoleDomain(role))
	}

	return domainRoles, nil
}

func (r RoleRepository) Save(role domain.Role) (domain.Role, error) {
	roleFound, err := (r.roleDao).FindByName(role.Name)
	if err != nil || roleFound.Id != "" {
		return domain.Role{}, fmt.Errorf("role already exists")
	}
	roleDto := ToRoleDTO(role)
	roleSave, err := (r.roleDao).Save(roleDto)
	if err != nil {
		return domain.Role{}, err
	}

	return ToRoleDomain(roleSave), nil
}

func (r RoleRepository) Update(role domain.Role) (domain.Role, error) {
	roleFound, err := (r.roleDao).FindById(role.Id)
	if err != nil || roleFound.Id == "" {
		return domain.Role{}, fmt.Errorf("role not found")
	}

	roleDto := ToRoleDTO(role)
	roleSave, err := (r.roleDao).Save(roleDto)
	if err != nil {
		return domain.Role{}, err
	}

	return ToRoleDomain(roleSave), nil
}

func ToRoleDomain(role dto.RoleDTO) domain.Role {
	return domain.Role{
		Base: domain.Base{
			Id: role.Id,
		},
		Name:        role.Name,
		Routes:      role.Routes,
		Permissions: ToPermission(role.Permissions),
	}
}

func ToRoleDTO(role domain.Role) dto.RoleDTO {
	return dto.RoleDTO{
		Name:        role.Name,
		Routes:      role.Routes,
		Permissions: ToPermissionDTO(role.Permissions),
	}
}
