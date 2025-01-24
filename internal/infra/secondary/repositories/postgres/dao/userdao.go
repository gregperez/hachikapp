package dao

import (
	"github.com/WilsonSayago/initModules"
	log "github.com/sirupsen/logrus"
	"jwt/base/internal/infra/config/database"
	"jwt/base/internal/infra/secondary/repositories/dto"
)

type UserDao struct {
	DbPgx4 *database.Database
}

func (c *UserDao) FindByUserId(userId string) ([]interface{}, error) {
	//TODO implement me
	panic("implement me")
}

func NewUserDaoInstance(db *database.Database) IUserDao {
	instance := initModules.GetInstance("UserDao", func() interface{} {
		return &UserDao{
			DbPgx4: db,
		}
	})
	return instance.(*UserDao)
}

func (c *UserDao) Save(user dto.UserDTO) error {
	query := "INSERT INTO public.users (name, email, password, is_admin) VALUES($1, $2, $3, $4)"
	err := (*c.DbPgx4).Exec(query,
		user.Name,
		user.Email,
		user.Password,
		user.IsAdmin,
	)
	if err != nil {
		log.Errorf("error in Save User, error: %s", err.Error())
		return err
	}
	return nil
}

func (c *UserDao) FindById(id string) (dto.UserDTO, error) {
	var email string
	var is_admin bool

	query := "SELECT email, is_admin FROM public.users WHERE id = $1;"
	err := (*c.DbPgx4).QueryRow(query, id).Scan(&email, &is_admin)
	if err != nil {
		log.Errorf("error in Save User, error: %s", err.Error())
		return dto.UserDTO{}, err
	}

	user := dto.UserDTO{
		BaseDTO: dto.BaseDTO{Id: id},
		Email:   email,
		IsAdmin: is_admin,
	}

	return user, nil
}

func (c *UserDao) FindAll() ([]dto.UserDTO, error) {

	var (
		email    string
		is_admin bool
	)
	query := "SELECT email, is_admin FROM public.users;"
	rows, err := (*c.DbPgx4).QueryRows(query)
	if err != nil {
		log.Errorf("error in FindAll users, error: %s", err.Error())
		return nil, err
	}

	list := make([]dto.UserDTO, 0)
	for rows.Next() {
		err = rows.Scan(&email, &is_admin)

		if err != nil {
			log.Errorf("error in Scan FindAll users, error: %s", err.Error())
			return nil, err
		}

		list = append(list, dto.UserDTO{
			Email:   email,
			IsAdmin: is_admin,
		})
	}

	return list, nil
}

func (c *UserDao) FindByEmail(email string) (dto.UserDTO, error) {
	var id, password string

	query := "SELECT id, password FROM public.users WHERE email = $1;"
	err := (*c.DbPgx4).QueryRow(query, email).Scan(&id, &password)
	if err != nil {
		log.Errorf("error in FindByEmail User, error: %s", err.Error())
		return dto.UserDTO{}, err
	}

	user := dto.UserDTO{
		BaseDTO:  dto.BaseDTO{Id: id},
		Password: password,
	}

	return user, nil
}
