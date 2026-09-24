package repositories

import (
	"fmt"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/dto"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/dao"

	"github.com/WilsonSayago/authBase/v3/core/domain"
	authBasePorts "github.com/WilsonSayago/authBase/v3/core/port"
	"github.com/WilsonSayago/initModules/v2"
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

func (r RoleRepository) FindAll(pageSize, offset int) ([]domain.Role, int, error) {
	total, err := (r.roleDao).Count()
	roles, err := (r.roleDao).FindAll(pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("errors.roles.get-all")
	}

	// convert roles to domain
	var domainRoles []domain.Role
	for _, role := range roles {
		domainRoles = append(domainRoles, ToRoleDomain(role))
	}

	return domainRoles, total, nil
}

func (r RoleRepository) Save(role domain.Role) (domain.Role, error) {
	roleFound, err := (r.roleDao).FindByName(role.Name)
	if err != nil || roleFound.Id != "" {
		return domain.Role{}, fmt.Errorf("errors.roles.already-exists")
	}
	roleDto := ToRoleDTO(role)
	roleSave, err := (r.roleDao).Save(roleDto)
	if err != nil {
		return domain.Role{}, fmt.Errorf("errors.roles.save")
	}

	return ToRoleDomain(roleSave), nil
}

func (r RoleRepository) Update(role domain.Role) error {
	roleFound, err := (r.roleDao).FindById(role.Id)
	if err != nil || roleFound.Id == "" {
		return fmt.Errorf("errors.roles.not-found")
	}

	roleDto := ToRoleDTO(role)
	err = (r.roleDao).Update(roleDto)
	if err != nil {
		return fmt.Errorf("errors.roles.update")
	}

	return nil
}

func (r RoleRepository) ChangeStatus(id string) error {
	roleFound, err := (r.roleDao).FindById(id)
	if err != nil || roleFound.Id == "" {
		return fmt.Errorf("errors.roles.not-found")
	}

	err = (r.roleDao).ChangeStatus(id)
	if err != nil {
		return fmt.Errorf("errors.roles.change-status")
	}

	return nil
}

func ToRoleDomain(role dto.RoleDTO) domain.Role {
	return domain.Role{
		Base: domain.Base{
			Id:     role.Id,
			Active: role.Active,
		},
		Name:        role.Name,
		Routes:      role.Routes,
		Permissions: ToPermission(role.Permissions),
	}
}

func ToRoleDTO(role domain.Role) dto.RoleDTO {
	return dto.RoleDTO{
		BaseDTO: dto.BaseDTO{
			Id: role.Id,
		},
		Name:        role.Name,
		Routes:      role.Routes,
		Permissions: ToPermissionDTO(role.Permissions),
	}
}
