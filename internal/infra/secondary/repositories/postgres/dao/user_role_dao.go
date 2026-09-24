package dao

import (
	"context"
	"fmt"

	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/initModules/v2"
)

type IUserRoleDao interface {
	Save(ctx context.Context, userId, roleId string) error
	DeleteAllByUserId(ctx context.Context, userId string) error
	SaveWithTx(ctx context.Context, tx database.Transaction, userId, roleId string) error
	DeleteAllByUserIdWithTx(ctx context.Context, tx database.Transaction, userId string) error
}

type UserRoleDao struct {
	db *database.PostgresPgx
}

func NewUserRoleDaoInstance(db *database.PostgresPgx) IUserRoleDao {
	return initModules.Once(func() *UserRoleDao {
		return &UserRoleDao{db: db}
	})
}

// NewUserRoleDaoForTest builds a DAO without process-wide Once (tests only).
func NewUserRoleDaoForTest(db *database.PostgresPgx) IUserRoleDao {
	return &UserRoleDao{db: db}
}

func (d *UserRoleDao) Save(ctx context.Context, userId, roleId string) error {
	return d.SaveWithTx(ctx, nil, userId, roleId)
}

func (d *UserRoleDao) SaveWithTx(ctx context.Context, tx database.Transaction, userId, roleId string) error {
	const q = `INSERT INTO public.users_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT (user_id, role_id) DO NOTHING`
	var err error
	if tx != nil {
		_, err = tx.Exec(ctx, q, userId, roleId)
	} else {
		_, err = d.db.Exec(ctx, q, userId, roleId)
	}
	if err != nil {
		return fmt.Errorf("save user role: %w", err)
	}
	return nil
}

func (d *UserRoleDao) DeleteAllByUserId(ctx context.Context, userId string) error {
	return d.DeleteAllByUserIdWithTx(ctx, nil, userId)
}

func (d *UserRoleDao) DeleteAllByUserIdWithTx(ctx context.Context, tx database.Transaction, userId string) error {
	const q = `DELETE FROM public.users_roles WHERE user_id = $1`
	var err error
	if tx != nil {
		_, err = tx.Exec(ctx, q, userId)
	} else {
		_, err = d.db.Exec(ctx, q, userId)
	}
	if err != nil {
		return fmt.Errorf("delete user roles: %w", err)
	}
	return nil
}
