package dao

import (
	"database/sql"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/dto"

	"github.com/WilsonSayago/initModules/v2"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type IUserDao interface {
	// Operaciones estándar sin transacción
	Save(user *dto.UserDTO) error
	FindById(id string) (dto.UserDTO, error)
	FindByUserId(userId string) ([]interface{}, error)
	FindAll(page, offset int) ([]dto.UserDTO, error)
	FindByEmail(email string) (dto.UserDTO, error)
	FindCredentialsByEmail(email string) (dto.UserDTO, error)
	Update(user dto.UserDTO) error
	ChangeStatus(id string) error
	Count() (int, error)
	CountAdmins() (int, error)
	UpdatePassword(id, passwordHash string) error

	// Operaciones con transacciones
	SaveWithTx(tx database.Transaction, user *dto.UserDTO) error
	UpdateWithTx(tx database.Transaction, user dto.UserDTO) error
	ChangeStatusWithTx(tx database.Transaction, id string) error

	// Gestión de transacciones
	BeginTx() (database.Transaction, error)
}

type UserDao struct {
	DbPgx4 *database.PostgresPgx
}

func NewUserDaoInstance(db *database.PostgresPgx) IUserDao {
	return initModules.Once(func() *UserDao {
		return &UserDao{DbPgx4: db}
	})
}

func (c *UserDao) FindByUserId(userId string) ([]interface{}, error) {
	//TODO implement me
	panic("implement me")
}

func (c *UserDao) Save(user *dto.UserDTO) error {
	query := "INSERT INTO public.users (name, email, password, is_admin) VALUES($1, $2, $3, $4) RETURNING id"
	var id string
	err := (*c.DbPgx4).QueryRow(query,
		user.Name,
		user.Email,
		user.Password,
		user.IsAdmin,
	).Scan(&id)
	if err != nil {
		log.Errorf("error in Save User, error: %s", err.Error())
		return err
	}
	user.Id = id
	return nil
}

func (c *UserDao) SaveWithTx(tx database.Transaction, user *dto.UserDTO) error {
	query := "INSERT INTO public.users (name, email, password, is_admin) VALUES($1, $2, $3, $4) RETURNING id"
	var id string
	err := tx.QueryRow(query,
		user.Name,
		user.Email,
		user.Password,
		user.IsAdmin,
	).Scan(&id)
	if err != nil {
		log.Errorf("error in Save User with Tx, error: %s", err.Error())
		return err
	}
	user.Id = id
	return nil
}

func (c *UserDao) FindById(id string) (dto.UserDTO, error) {
	var name, email string
	var isAdmin, active bool

	query := "SELECT name, email, is_admin, active FROM public.users WHERE id = $1;"
	err := (*c.DbPgx4).QueryRow(query, id).Scan(&name, &email, &isAdmin, &active)
	if err != nil {
		log.Errorf("error in Save User, error: %s", err.Error())
		return dto.UserDTO{}, err
	}

	user := dto.UserDTO{
		BaseDTO: dto.BaseDTO{
			Id:     id,
			Active: active,
		},
		Name:    name,
		Email:   email,
		IsAdmin: isAdmin,
	}

	return user, nil
}

func (c *UserDao) FindAll(pageSize, offset int) ([]dto.UserDTO, error) {
	var (
		id, name, email string
		isAdmin, active bool
	)
	query := "SELECT id, name, email, is_admin, active FROM public.users ORDER BY id LIMIT $1 OFFSET $2;"
	rows, err := (*c.DbPgx4).QueryRows(query, pageSize, offset)
	if err != nil {
		log.Errorf("error in FindAll users, error: %s", err.Error())
		return nil, err
	}

	list := make([]dto.UserDTO, 0)
	for rows.Next() {
		err = rows.Scan(&id, &name, &email, &isAdmin, &active)

		if err != nil {
			log.Errorf("error in Scan FindAll users, error: %s", err.Error())
			return nil, err
		}

		list = append(list, dto.UserDTO{
			BaseDTO: dto.BaseDTO{
				Id:     id,
				Active: active,
			},
			Name:    name,
			Email:   email,
			IsAdmin: isAdmin,
		})
	}

	return list, nil
}

func (c *UserDao) FindByEmail(email string) (dto.UserDTO, error) {
	var id, password string

	query := "SELECT id, password FROM public.users WHERE email = $1;"
	err := (*c.DbPgx4).QueryRow(query, email).Scan(&id, &password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.UserDTO{}, nil
		}
		log.Errorf("error in FindByEmail User, error: %s", err.Error())
		return dto.UserDTO{}, err
	}

	return dto.UserDTO{
		BaseDTO:  dto.BaseDTO{Id: id},
		Password: password,
	}, nil
}

func (c *UserDao) FindCredentialsByEmail(email string) (dto.UserDTO, error) {
	var id, password, foundEmail string
	var active bool
	query := "SELECT id, email, password, active FROM public.users WHERE email = $1;"
	err := (*c.DbPgx4).QueryRow(query, email).Scan(&id, &foundEmail, &password, &active)
	if err != nil {
		return dto.UserDTO{}, err
	}
	return dto.UserDTO{
		BaseDTO:  dto.BaseDTO{Id: id, Active: active},
		Email:    foundEmail,
		Password: password,
	}, nil
}

func (c *UserDao) Update(user dto.UserDTO) error {
	query := "UPDATE public.users SET name = $1, email = $2, is_admin = $3 WHERE id = $4;"
	err := (*c.DbPgx4).Exec(query,
		user.Name,
		user.Email,
		user.IsAdmin,
		user.Id,
	)
	if err != nil {
		log.Errorf("error in Update User, error: %s", err.Error())
		return err
	}
	return nil
}

func (c *UserDao) UpdateWithTx(tx database.Transaction, user dto.UserDTO) error {
	query := "UPDATE public.users SET name = $1, email = $2, is_admin = $3 WHERE id = $4;"
	err := tx.Exec(query,
		user.Name,
		user.Email,
		user.IsAdmin,
		user.Id,
	)
	if err != nil {
		log.Errorf("error in Update User with Tx, error: %s", err.Error())
		return err
	}
	return nil
}

func (c *UserDao) ChangeStatus(id string) error {
	query := "UPDATE public.users SET active = NOT active WHERE id = $1;"
	err := (*c.DbPgx4).Exec(query, id)
	if err != nil {
		log.Errorf("error in Disable User, error: %s", err.Error())
		return err
	}
	return nil
}

func (c *UserDao) ChangeStatusWithTx(tx database.Transaction, id string) error {
	query := "UPDATE public.users SET active = NOT active WHERE id = $1;"
	err := tx.Exec(query, id)
	if err != nil {
		log.Errorf("error in Disable User with Tx, error: %s", err.Error())
		return err
	}
	return nil
}

func (c *UserDao) Count() (int, error) {
	var totalRow int
	query := "SELECT COUNT(*) FROM public.users;"
	err := (*c.DbPgx4).QueryRow(query).Scan(&totalRow)
	if err != nil {
		log.Errorf("error in Count User, error: %s", err.Error())
		return 0, err
	}
	return totalRow, nil
}

func (c *UserDao) CountAdmins() (int, error) {
	var totalRow int
	query := "SELECT COUNT(*) FROM public.users WHERE is_admin IS TRUE AND active IS TRUE;"
	err := (*c.DbPgx4).QueryRow(query).Scan(&totalRow)
	if err != nil {
		log.Errorf("error in CountAdmins, error: %s", err.Error())
		return 0, err
	}
	return totalRow, nil
}

func (c *UserDao) UpdatePassword(id, passwordHash string) error {
	query := "UPDATE public.users SET password = $1 WHERE id = $2;"
	err := (*c.DbPgx4).Exec(query, passwordHash, id)
	if err != nil {
		log.Errorf("error in UpdatePassword, error: %s", err.Error())
		return err
	}
	return nil
}

func (c *UserDao) BeginTx() (database.Transaction, error) {
	tx, err := c.DbPgx4.BeginTx()
	if err != nil {
		log.Errorf("error in BeginTx, error: %s", err.Error())
		return nil, err
	}
	return tx, nil
}
