package dao

import (
	log "github.com/sirupsen/logrus"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/infra/primary/config/database"
	"jwt/base/internal/infra/secondary/repositories"
	"sync"
)

var (
	serviceSingleton    *repositories.IGenericDAO
	serviceInstanceOnce sync.Once
)

type UserDao struct {
	DbPgx4 *database.Database
}

func NewUserDaoInstance(db *database.Database) *repositories.IGenericDAO {
	serviceInstanceOnce.Do(func() {
		var valueDao repositories.IGenericDAO = &UserDao{db}
		serviceSingleton = &valueDao
	})
	return serviceSingleton
}

func (c *UserDao) Save(user interface{}) error {
	query := "INSERT INTO public.countries (email, password, permissions, is_admin) VALUES($1, $2, $3, $4)"
	err := (*c.DbPgx4).Exec(query,
		user.(domain.User).Email,
		user.(domain.User).Password,
		user.(domain.User).Permissions,
		user.(domain.User).IsAdmin,
	)
	if err != nil {
		log.Errorf("error in Save User, error: %s", err.Error())
		return err
	}
	return nil
}

func (c *UserDao) FindOne(id string) (interface{}, error) {
	return nil, nil
}
