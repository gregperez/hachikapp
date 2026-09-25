package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/dto"
	"github.com/WilsonSayago/initModules/v2"
	"github.com/jackc/pgx/v5"
)

type IUserDao interface {
	Save(ctx context.Context, user *dto.UserDTO) error
	FindById(ctx context.Context, id string) (dto.UserDTO, error)
	FindAll(ctx context.Context, request authBaseDomain.PageRequest) ([]dto.UserDTO, bool, error)
	FindByEmail(ctx context.Context, email string) (dto.UserDTO, error)
	FindCredentialsByEmail(ctx context.Context, email string) (dto.UserDTO, error)
	Update(ctx context.Context, user dto.UserDTO) error
	SetActive(ctx context.Context, id string, active bool) error
	CountAdmins(ctx context.Context) (int, error)
	CountAdminsWithTx(ctx context.Context, tx database.Transaction) (int, error)
	UpdatePassword(ctx context.Context, id, passwordHash string) error
	// DemoteAdminIfNotLast clears is_admin only when another active admin remains.
	DemoteAdminIfNotLast(ctx context.Context, id string) error
	// DeactivateAdminIfNotLast sets active=false only when another active admin remains.
	DeactivateAdminIfNotLast(ctx context.Context, id string) error

	SaveWithTx(ctx context.Context, tx database.Transaction, user *dto.UserDTO) error
	UpdateWithTx(ctx context.Context, tx database.Transaction, user dto.UserDTO) error
	SetActiveWithTx(ctx context.Context, tx database.Transaction, id string, active bool) error
	UpdatePasswordWithTx(ctx context.Context, tx database.Transaction, id, passwordHash string) error
	DeactivateAdminIfNotLastWithTx(ctx context.Context, tx database.Transaction, id string) error

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
		if mapped := mapUniqueViolation(err); errors.Is(mapped, domain.ErrConflict) {
			return domain.ErrConflict
		}
		return fmt.Errorf("save user: %w", err)
	}
	user.Id = id
	return nil
}

func (c *UserDao) FindById(ctx context.Context, id string) (dto.UserDTO, error) {
	const q = `SELECT name, email, is_admin, active, created_at, updated_at FROM public.users WHERE id = $1`
	var name, email string
	var isAdmin, active bool
	var createdAt, updatedAt time.Time
	err := c.db.QueryRow(ctx, q, id).Scan(&name, &email, &isAdmin, &active, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.UserDTO{}, domain.ErrNotFound
		}
		return dto.UserDTO{}, fmt.Errorf("find user by id: %w", err)
	}
	return dto.UserDTO{
		BaseDTO: dto.BaseDTO{
			Id:        id,
			Active:    active,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
		Name:    name,
		Email:   email,
		IsAdmin: isAdmin,
	}, nil
}

func (c *UserDao) FindAll(ctx context.Context, request authBaseDomain.PageRequest) ([]dto.UserDTO, bool, error) {
	const firstPage = `SELECT id, name, email, is_admin, active, created_at, updated_at
FROM public.users
ORDER BY created_at DESC, id DESC
LIMIT $1`
	const nextPage = `SELECT id, name, email, is_admin, active, created_at, updated_at
FROM public.users
WHERE (created_at, id) < ($1, $2)
ORDER BY created_at DESC, id DESC
LIMIT $3`

	limit := request.Limit + 1
	var rows pgx.Rows
	var err error
	if request.After == nil {
		rows, err = c.db.Query(ctx, firstPage, limit)
	} else {
		rows, err = c.db.Query(ctx, nextPage, request.After.CreatedAt, request.After.ID, limit)
	}
	if err != nil {
		return nil, false, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	list := make([]dto.UserDTO, 0)
	for rows.Next() {
		var item dto.UserDTO
		if err := rows.Scan(
			&item.Id,
			&item.Name,
			&item.Email,
			&item.IsAdmin,
			&item.Active,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, false, fmt.Errorf("scan user: %w", err)
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate users: %w", err)
	}
	hasNext := len(list) > request.Limit
	if hasNext {
		list = list[:request.Limit]
	}
	return list, hasNext, nil
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

func (c *UserDao) CountAdmins(ctx context.Context) (int, error) {
	var total int
	err := c.db.QueryRow(ctx, `SELECT COUNT(*) FROM public.users WHERE is_admin IS TRUE AND active IS TRUE`).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return total, nil
}

func (c *UserDao) CountAdminsWithTx(ctx context.Context, tx database.Transaction) (int, error) {
	var total int
	err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM public.users WHERE is_admin IS TRUE AND active IS TRUE`).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return total, nil
}

func (c *UserDao) DemoteAdminIfNotLast(ctx context.Context, id string) error {
	tx, err := c.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin demote admin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(87420011)`); err != nil {
		return fmt.Errorf("lock demote admin: %w", err)
	}
	// Serialize against concurrent demote/deactivate of other admins.
	if _, err := tx.Exec(ctx, `
SELECT id FROM public.users
WHERE is_admin IS TRUE AND active IS TRUE
FOR UPDATE`); err != nil {
		return fmt.Errorf("lock admin rows: %w", err)
	}

	const q = `
UPDATE public.users
SET is_admin = false
WHERE id = $1
  AND is_admin IS TRUE
  AND (
        SELECT COUNT(*) FROM public.users
        WHERE is_admin IS TRUE AND active IS TRUE AND id <> $1
      ) > 0`
	tag, err := tx.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("demote admin: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var isAdmin, active bool
		err := tx.QueryRow(ctx, `SELECT is_admin, active FROM public.users WHERE id = $1`, id).Scan(&isAdmin, &active)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("load user after demote: %w", err)
		}
		if isAdmin && active {
			return domain.ErrLastAdmin
		}
		return nil
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit demote admin: %w", err)
	}
	return nil
}

func (c *UserDao) DeactivateAdminIfNotLast(ctx context.Context, id string) error {
	tx, err := c.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin deactivate admin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := c.DeactivateAdminIfNotLastWithTx(ctx, tx, id); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit deactivate admin: %w", err)
	}
	return nil
}

func (c *UserDao) DeactivateAdminIfNotLastWithTx(ctx context.Context, tx database.Transaction, id string) error {
	if tx == nil {
		return fmt.Errorf("deactivate admin transaction is nil")
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(87420011)`); err != nil {
		return fmt.Errorf("lock deactivate admin: %w", err)
	}
	if _, err := tx.Exec(ctx, `
SELECT id FROM public.users
WHERE is_admin IS TRUE AND active IS TRUE
FOR UPDATE`); err != nil {
		return fmt.Errorf("lock admin rows: %w", err)
	}

	const q = `
UPDATE public.users
SET active = false
WHERE id = $1
  AND active IS TRUE
  AND (
        is_admin IS NOT TRUE
        OR (
             SELECT COUNT(*) FROM public.users
             WHERE is_admin IS TRUE AND active IS TRUE AND id <> $1
           ) > 0
      )`
	tag, err := tx.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("deactivate user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var isAdmin, active bool
		err := tx.QueryRow(ctx, `SELECT is_admin, COALESCE(active,true) FROM public.users WHERE id = $1`, id).Scan(&isAdmin, &active)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("load user after deactivate: %w", err)
		}
		if !active {
			return nil
		}
		if isAdmin {
			return domain.ErrLastAdmin
		}
		return domain.ErrNotFound
	}
	return nil
}

func (c *UserDao) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	return c.UpdatePasswordWithTx(ctx, nil, id, passwordHash)
}

func (c *UserDao) UpdatePasswordWithTx(
	ctx context.Context,
	tx database.Transaction,
	id, passwordHash string,
) error {
	const q = `UPDATE public.users SET password = $1 WHERE id = $2`
	var tag interface{ RowsAffected() int64 }
	var err error
	if tx != nil {
		tag, err = tx.Exec(ctx, q, passwordHash, id)
	} else {
		tag, err = c.db.Exec(ctx, q, passwordHash, id)
	}
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
