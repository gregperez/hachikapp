package dao

import (
	"context"
	"errors"
	"fmt"

	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/dto"
	"github.com/WilsonSayago/initModules/v2"
	"github.com/jackc/pgx/v5"
)

type IUserDao interface {
	Save(ctx context.Context, user *dto.UserDTO) error
	FindById(ctx context.Context, id string) (dto.UserDTO, error)
	FindAll(ctx context.Context, pageSize, offset int) ([]dto.UserDTO, error)
	FindByEmail(ctx context.Context, email string) (dto.UserDTO, error)
	FindCredentialsByEmail(ctx context.Context, email string) (dto.UserDTO, error)
	Update(ctx context.Context, user dto.UserDTO) error
	SetActive(ctx context.Context, id string, active bool) error
	Count(ctx context.Context) (int, error)
	CountAdmins(ctx context.Context) (int, error)
	UpdatePassword(ctx context.Context, id, passwordHash string) error

	SaveWithTx(ctx context.Context, tx database.Transaction, user *dto.UserDTO) error
	UpdateWithTx(ctx context.Context, tx database.Transaction, user dto.UserDTO) error
	SetActiveWithTx(ctx context.Context, tx database.Transaction, id string, active bool) error

	BeginTx(ctx context.Context) (database.Transaction, error)
}

type UserDao struct {
	db *database.PostgresPgx
}

func NewUserDaoInstance(db *database.PostgresPgx) IUserDao {
	return initModules.Once(func() *UserDao {
		return &UserDao{db: db}
	})
}

// NewUserDaoForTest builds a DAO without process-wide Once (tests only).
func NewUserDaoForTest(db *database.PostgresPgx) IUserDao {
	return &UserDao{db: db}
}

func (c *UserDao) BeginTx(ctx context.Context) (database.Transaction, error) {
	return c.db.BeginTx(ctx)
}

func (c *UserDao) Save(ctx context.Context, user *dto.UserDTO) error {
	return c.SaveWithTx(ctx, nil, user)
}

func (c *UserDao) SaveWithTx(ctx context.Context, tx database.Transaction, user *dto.UserDTO) error {
	const q = `INSERT INTO public.users (name, email, password, is_admin) VALUES($1, $2, $3, $4) RETURNING id`
	var id string
	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, q, user.Name, user.Email, user.Password, user.IsAdmin).Scan(&id)
	} else {
		err = c.db.QueryRow(ctx, q, user.Name, user.Email, user.Password, user.IsAdmin).Scan(&id)
	}
	if err != nil {
		return fmt.Errorf("save user: %w", err)
	}
	user.Id = id
	return nil
}

func (c *UserDao) FindById(ctx context.Context, id string) (dto.UserDTO, error) {
	const q = `SELECT name, email, is_admin, active FROM public.users WHERE id = $1`
	var name, email string
	var isAdmin, active bool
	err := c.db.QueryRow(ctx, q, id).Scan(&name, &email, &isAdmin, &active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.UserDTO{}, domain.ErrNotFound
		}
		return dto.UserDTO{}, fmt.Errorf("find user by id: %w", err)
	}
	return dto.UserDTO{
		BaseDTO: dto.BaseDTO{Id: id, Active: active},
		Name:    name,
		Email:   email,
		IsAdmin: isAdmin,
	}, nil
}

func (c *UserDao) FindAll(ctx context.Context, pageSize, offset int) ([]dto.UserDTO, error) {
	const q = `SELECT id, name, email, is_admin, active FROM public.users ORDER BY id LIMIT $1 OFFSET $2`
	rows, err := c.db.Query(ctx, q, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	list := make([]dto.UserDTO, 0)
	for rows.Next() {
		var id, name, email string
		var isAdmin, active bool
		if err := rows.Scan(&id, &name, &email, &isAdmin, &active); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		list = append(list, dto.UserDTO{
			BaseDTO: dto.BaseDTO{Id: id, Active: active},
			Name:    name,
			Email:   email,
			IsAdmin: isAdmin,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return list, nil
}

func (c *UserDao) FindByEmail(ctx context.Context, email string) (dto.UserDTO, error) {
	const q = `SELECT id, password FROM public.users WHERE email = $1`
	var id, password string
	err := c.db.QueryRow(ctx, q, email).Scan(&id, &password)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.UserDTO{}, nil
		}
		return dto.UserDTO{}, fmt.Errorf("find user by email: %w", err)
	}
	return dto.UserDTO{BaseDTO: dto.BaseDTO{Id: id}, Password: password}, nil
}

func (c *UserDao) FindCredentialsByEmail(ctx context.Context, email string) (dto.UserDTO, error) {
	const q = `SELECT id, email, password, active FROM public.users WHERE email = $1`
	var id, foundEmail, password string
	var active bool
	err := c.db.QueryRow(ctx, q, email).Scan(&id, &foundEmail, &password, &active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.UserDTO{}, domain.ErrNotFound
		}
		return dto.UserDTO{}, fmt.Errorf("find credentials: %w", err)
	}
	return dto.UserDTO{
		BaseDTO:  dto.BaseDTO{Id: id, Active: active},
		Email:    foundEmail,
		Password: password,
	}, nil
}

func (c *UserDao) Update(ctx context.Context, user dto.UserDTO) error {
	return c.UpdateWithTx(ctx, nil, user)
}

func (c *UserDao) UpdateWithTx(ctx context.Context, tx database.Transaction, user dto.UserDTO) error {
	const q = `UPDATE public.users SET name = $1, email = $2, is_admin = $3 WHERE id = $4`
	var tag interface{ RowsAffected() int64 }
	var err error
	if tx != nil {
		tag, err = tx.Exec(ctx, q, user.Name, user.Email, user.IsAdmin, user.Id)
	} else {
		tag, err = c.db.Exec(ctx, q, user.Name, user.Email, user.IsAdmin, user.Id)
	}
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (c *UserDao) SetActive(ctx context.Context, id string, active bool) error {
	return c.SetActiveWithTx(ctx, nil, id, active)
}

func (c *UserDao) SetActiveWithTx(ctx context.Context, tx database.Transaction, id string, active bool) error {
	const q = `UPDATE public.users SET active = $2 WHERE id = $1`
	var tag interface{ RowsAffected() int64 }
	var err error
	if tx != nil {
		tag, err = tx.Exec(ctx, q, id, active)
	} else {
		tag, err = c.db.Exec(ctx, q, id, active)
	}
	if err != nil {
		return fmt.Errorf("set user active: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (c *UserDao) Count(ctx context.Context) (int, error) {
	var total int
	if err := c.db.QueryRow(ctx, `SELECT COUNT(*) FROM public.users`).Scan(&total); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return total, nil
}

func (c *UserDao) CountAdmins(ctx context.Context) (int, error) {
	var total int
	err := c.db.QueryRow(ctx, `SELECT COUNT(*) FROM public.users WHERE is_admin IS TRUE AND active IS TRUE`).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return total, nil
}

func (c *UserDao) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	tag, err := c.db.Exec(ctx, `UPDATE public.users SET password = $1 WHERE id = $2`, passwordHash, id)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
