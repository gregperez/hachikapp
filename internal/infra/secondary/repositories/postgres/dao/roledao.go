package dao

import (
	"fmt"
	domain "github.com/WilsonSayago/authBase/core/domains"
	"github.com/WilsonSayago/initModules"
	log "github.com/sirupsen/logrus"
	"jwt/base/internal/infra/primary/config/database"
	"jwt/base/internal/infra/secondary/repositories/dto"
)

type RoleDao struct {
	DbPgx4 *database.Database
}

func NewRoleDaoInstance(db *database.Database) IRoleDao {
	instance := initModules.GetInstance("RoleDao", func() interface{} {
		return &RoleDao{
			DbPgx4: db,
		}
	})
	return instance.(*RoleDao)
}

func (c *RoleDao) FindAllByUserId(userId string) ([]dto.RoleDTO, error) {
	var id, name string
	var routes []domain.RoutePathEnum
	var permissions []dto.PermissionDTO

	query := "select r.id, r.name, r.routes, r.permissions " +
		" from public.roles r " +
		" inner join users_roles ur on ur.role_id = r.id " +
		" where ur.user_id = $1;"
	rows, err := (*c.DbPgx4).QueryRows(query, userId)

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
