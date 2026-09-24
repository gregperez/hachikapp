package dao

import (
	"database/sql"
	"fmt"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/dto"

	"github.com/pkg/errors"

	domain "github.com/WilsonSayago/authBase/v3/core/domain"
	"github.com/WilsonSayago/initModules/v2"
	log "github.com/sirupsen/logrus"
)

type IRoleDao interface {
	FindAllByUserId(userId string) ([]dto.RoleDTO, error)
	FindById(userId string) (dto.RoleDTO, error)
	FindAll(pageSize, offset int) ([]dto.RoleDTO, error)
	Save(role dto.RoleDTO) (dto.RoleDTO, error)
	Update(role dto.RoleDTO) error
	FindByName(name string) (dto.RoleDTO, error)
	ChangeStatus(id string) error
	Count() (int, error)
}

type RoleDao struct {
	DbPgx4 *database.Database
}

func NewRoleDaoInstance(db *database.Database) IRoleDao {
	return initModules.Once(func() *RoleDao {
		return &RoleDao{DbPgx4: db}
	})
}

func (r *RoleDao) FindAllByUserId(userId string) ([]dto.RoleDTO, error) {
	var id, name string
	var routes []domain.RoutePathEnum
	var permissions []dto.PermissionDTO

	query := "select r.id, r.name, r.routes, r.permissions " +
		" from public.roles r " +
		" inner join users_roles ur on ur.role_id = r.id " +
		" where ur.user_id = $1;"
	rows, err := (*r.DbPgx4).QueryRows(query, userId)

	if err != nil {
		log.Errorf("error in FindAll Roles by userId, error: %s", err.Error())
		return nil, err
	}

	list := make([]dto.RoleDTO, 0)
	for rows.Next() {
		err = rows.Scan(&id, &name, &routes, &permissions)
		if err != nil {
			return nil, fmt.Errorf("could not get roles: %v", err)
		}
		list = append(list, dto.RoleDTO{
			BaseDTO: dto.BaseDTO{
				Id: id,
			},
			Name:        name,
			Routes:      routes,
			Permissions: permissions,
		})
	}

	return list, nil
}

func (r *RoleDao) FindById(roleId string) (dto.RoleDTO, error) {
	var id, name string
	var routes []domain.RoutePathEnum
	var permissions []dto.PermissionDTO
	var active bool

	query := "select r.id, r.name, r.routes, r.permissions, active " +
		" from public.roles r " +
		" where r.id = $1;"
	row := (*r.DbPgx4).QueryRow(query, roleId)

	err := row.Scan(&id, &name, &routes, &permissions, &active)
	if err != nil {
		log.Errorf("error in FindById Role, error: %s", err.Error())
		return dto.RoleDTO{}, err
	}

	data := dto.RoleDTO{
		BaseDTO: dto.BaseDTO{
			Id:     id,
			Active: active,
		},
		Name:        name,
		Routes:      routes,
		Permissions: permissions,
	}

	return data, nil
}

func (r *RoleDao) FindAll(pageSize, offset int) ([]dto.RoleDTO, error) {
	var id, name string
	var routes []domain.RoutePathEnum
	var permissions []dto.PermissionDTO
	var active bool

	query := "select r.id, r.name, r.routes, r.permissions, active " +
		" from public.roles r ORDER BY r.id LIMIT $1 OFFSET $2;"
	rows, err := (*r.DbPgx4).QueryRows(query, pageSize, offset)

	if err != nil {
		log.Errorf("error in FindAll Roles, error: %s", err.Error())
		return nil, err
	}

	list := make([]dto.RoleDTO, 0)
	for rows.Next() {
		err = rows.Scan(&id, &name, &routes, &permissions, &active)
		if err != nil {
			return nil, fmt.Errorf("could not get roles: %v", err)
		}

		list = append(list, dto.RoleDTO{
			BaseDTO: dto.BaseDTO{
				Id:     id,
				Active: active,
			},
			Name:        name,
			Routes:      routes,
			Permissions: permissions,
		})
	}

	return list, nil
}

func (r *RoleDao) Save(role dto.RoleDTO) (dto.RoleDTO, error) {
	query := "insert into public.roles (name, routes, permissions) values ($1, $2, $3) returning id;"
	row := (*r.DbPgx4).QueryRow(query, role.Name, role.Routes, role.Permissions)

	var id string
	err := row.Scan(&id)
	if err != nil {
		log.Errorf("error in Save Role, error: %s", err.Error())
		return dto.RoleDTO{}, err
	}

	return dto.RoleDTO{
		BaseDTO: dto.BaseDTO{
			Id: id,
		},
		Name:        role.Name,
		Routes:      role.Routes,
		Permissions: role.Permissions,
	}, nil
}

func (r *RoleDao) FindByName(name string) (dto.RoleDTO, error) {
	var id string
	var routes []domain.RoutePathEnum
	var permissions []dto.PermissionDTO
	var active bool

	query := "select r.id, r.name, r.routes, r.permissions, active " +
		" from public.roles r " +
		" where r.name = $1;"
	row := (*r.DbPgx4).QueryRow(query, name)

	err := row.Scan(&id, &name, &routes, &permissions, &active)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.RoleDTO{}, nil
		} else {
			log.Errorf("error in FindByName Role, error: %s", err.Error())
			return dto.RoleDTO{}, err
		}
	}

	return dto.RoleDTO{
		BaseDTO: dto.BaseDTO{
			Id:     id,
			Active: active,
		},
		Name:        name,
		Routes:      routes,
		Permissions: permissions,
	}, nil
}

func (r *RoleDao) ChangeStatus(id string) error {
	query := "UPDATE public.roles SET active = NOT active WHERE id = $1;"
	err := (*r.DbPgx4).Exec(query, id)
	if err != nil {
		log.Errorf("error in Disable Role, error: %s", err.Error())
		return err
	}
	return nil
}

func (r *RoleDao) Count() (int, error) {
	query := "select count(id) from public.roles;"
	row := (*r.DbPgx4).QueryRow(query)

	var count int
	err := row.Scan(&count)
	if err != nil {
		log.Errorf("error in Count Roles, error: %s", err.Error())
		return 0, err
	}

	return count, nil
}

func (r *RoleDao) Update(role dto.RoleDTO) error {
	query := "update public.roles set name = $1, permissions = $2 where id = $3;"
	err := (*r.DbPgx4).Exec(query, role.Name, role.Permissions, role.Id)
	if err != nil {
		log.Errorf("error in Update Role, error: %s", err.Error())
		return err
	}
	return nil
}
