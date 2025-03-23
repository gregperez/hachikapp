package dao

import (
	"jwt/base/internal/infra/config/database"

	"github.com/WilsonSayago/initModules"
	log "github.com/sirupsen/logrus"
)

type IUserRoleDao interface {
	// Operaciones estándar sin transacción
	Save(userId, roleId string) error
	DeleteAllByUserId(userId string) error

	// Operaciones con transacciones
	SaveWithTx(tx database.Transaction, userId, roleId string) error
	DeleteAllByUserIdWithTx(tx database.Transaction, userId string) error

	// Gestión de transacciones
	BeginTx() (database.Transaction, error)
}

type UserRoleDao struct {
	DbPgx4 *database.PostgresPgx
}

func NewUserRoleDaoInstance(db *database.PostgresPgx) IUserRoleDao {
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

func (userRoleDao *UserRoleDao) SaveWithTx(tx database.Transaction, userId, roleId string) error {
	query := "insert into public.users_roles (user_id, role_id) values ($1, $2);"
	err := tx.Exec(query, userId, roleId)
	if err != nil {
		log.Errorf("error in Save User Role with Tx, error: %s", err.Error())
		return err
	}

	return nil
}

func (userRoleDao *UserRoleDao) DeleteAllByUserIdWithTx(tx database.Transaction, userId string) error {
	query := "delete from public.users_roles where user_id = $1;"
	err := tx.Exec(query, userId)
	if err != nil {
		log.Errorf("error in DeleteAllByUserId with Tx, error: %s", err.Error())
		return err
	}

	return nil
}

func (userRoleDao *UserRoleDao) BeginTx() (database.Transaction, error) {
	tx, err := userRoleDao.DbPgx4.BeginTx()
	if err != nil {
		log.Errorf("error in BeginTx, error: %s", err.Error())
		return nil, err
	}
	return tx, nil
}
