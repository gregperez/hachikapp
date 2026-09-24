package dao

import (
	"database/sql"
	"fmt"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/dto"

	"github.com/WilsonSayago/initModules/v2"
	log "github.com/sirupsen/logrus"
)

type IPermissionDao interface {
	FindAllByUserId(userId string) ([]dto.PermissionDTO, error)
}

type PermissionDao struct {
	DbPgx4 *database.Database
}

func NewPermissionDaoInstance(db *database.Database) IPermissionDao {
	instance := initModules.GetInstance("PermissionDao", func() interface{} {
		return &PermissionDao{
			DbPgx4: db,
		}
	})
	return instance.(*PermissionDao)
}

func (c *PermissionDao) FindAllByUserId(id string) ([]dto.PermissionDTO, error) {
	var entity string
	var create, read, update, deleteattr sql.NullBool

	query := "SELECT p.entity, p.create, p.read, p.update, p.delete " +
		" FROM public.permissions p " +
		" inner join public.users_permissions up " +
		" on up.permission_id = p.id " +
		" where up.user_id = $1;"
	rows, err := (*c.DbPgx4).QueryRows(query, id)
	if err != nil {
		log.Errorf("error in Save User, error: %s", err.Error())
		return nil, err
	}

	list := make([]dto.PermissionDTO, 0)
	for rows.Next() {
		err = rows.Scan(&entity, &create, &read, &update, &deleteattr)
		if err != nil {
			return nil, fmt.Errorf("could not get permissions: %v", err)
		}
		list = append(list, dto.PermissionDTO{
			Entity: entity,
			Create: create.Bool,
			Read:   read.Bool,
			Update: update.Bool,
			Delete: deleteattr.Bool,
		})
	}

	return list, nil
}
