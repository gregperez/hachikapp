package dao

import (
	"jwt/base/internal/infra/config/database"

	"github.com/WilsonSayago/initModules"
	log "github.com/sirupsen/logrus"
)

type IUserRoleDao interface {
	Save(userId, roleId string) error
	DeleteAllByUserId(userId string) error
}

type UserRoleDao struct {
	DbPgx4 *database.Database
}

func NewUserRoleDaoInstance(db *database.Database) IUserRoleDao {
	instance := initModules.GetInstance("UserRoleDao", func() interface{} {
		return &UserRoleDao{
			DbPgx4: db,
		}
	})
	return instance.(*UserRoleDao)
}

func (userRoleDao *UserRoleDao) Save(userId, roleId string) error {
	query := "insert into public.users_roles (user_id, role_id) values ($1, $2);"
	err := (*userRoleDao.DbPgx4).Exec(query, userId, roleId)
	if err != nil {
		log.Errorf("error in Save User Role, error: %s", err.Error())
		return err
	}

	return nil
}

func (userRoleDao *UserRoleDao) DeleteAllByUserId(userId string) error {
	query := "delete from public.users_roles where user_id = $1;"
	err := (*userRoleDao.DbPgx4).Exec(query, userId)
	if err != nil {
		log.Errorf("error in DeleteAllByUserId, error: %s", err.Error())
		return err
	}

	return nil
}
