package dao

import (
	"fmt"
	middlewareDomain "github.com/WilsonSayago/middleware/core/domains"
	log "github.com/sirupsen/logrus"
	"jwt/base/internal/infra/primary/config/database"
	"jwt/base/internal/infra/secondary/repositories"
	"jwt/base/internal/infra/secondary/repositories/dto"
	"sync"
)

var (
	permissionServiceSingleton    *repositories.IGenericDAO
	permissionServiceInstanceOnce sync.Once
)

type PermissionDao struct {
	DbPgx4 *database.Database
}

func NewPermissionDaoInstance(db *database.Database) *repositories.IGenericDAO {
	permissionServiceInstanceOnce.Do(func() {
		var valueDao repositories.IGenericDAO = &PermissionDao{db}
		permissionServiceSingleton = &valueDao
	})
	return permissionServiceSingleton
}

func (c *PermissionDao) Save(user interface{}) error {
	return nil
}

func (c *PermissionDao) FindById(id string) (interface{}, error) {
	return nil, nil
}

func (c *PermissionDao) FindByUserId(id string) ([]interface{}, error) {
	var entity string
	
	query := "SELECT p.entity " +
		" FROM public.permissions p " +
		" inner join public.users_permissions up " +
		" on up.permission_id = p.id " +
		" where up.user_id = $1;"
	rows, err := (*c.DbPgx4).QueryRows(query, id)
	if err != nil {
		log.Errorf("error in Save User, error: %s", err.Error())
		return nil, err
	}
	
	list := make([]interface{}, 0)
	for rows.Next() {
		err = rows.Scan(&entity)
		if err != nil {
			return nil, fmt.Errorf("could not get permissions: %v", err)
		}
		list = append(list, dto.PermissionDTO{
			Entity: middlewareDomain.EntityEnum(entity),
		})
	}
	
	return list, nil
}
