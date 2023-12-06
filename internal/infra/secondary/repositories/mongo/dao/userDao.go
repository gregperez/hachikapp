package dao

import (
	log "github.com/sirupsen/logrus"
	"jwt/base/internal/infra/primary/config/database"
	"jwt/base/internal/infra/secondary/repositories"
	"jwt/base/internal/infra/secondary/repositories/dto"
	"sync"
)

var (
	userSingleton    *repositories.IGenericDAO
	userInstanceOnce sync.Once
)

type UserDao struct {
	MongoDb *database.IMongo
}

func NewUserMongoDaoInstance(db *database.IMongo) *repositories.IGenericDAO {
	userInstanceOnce.Do(func() {
		var valueDao repositories.IGenericDAO = &UserDao{db}
		userSingleton = &valueDao
	})
	return userSingleton
}

func (cDao *UserDao) Save(c interface{}) error {
	data, err := (*cDao.MongoDb).Save("users", c.(dto.UserDTO))
	if err != nil {
		return err
	}
	log.Infof("data: %v", data)
	return nil
}

func (cDao *UserDao) FindById(id string) (interface{}, error) {
	return nil, nil
}
