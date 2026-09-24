package dao

import (
	"context"
	"errors"
	"fmt"

	authBaseDomain "github.com/WilsonSayago/authBase/v3/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/dto"
	"github.com/WilsonSayago/initModules/v2"
	"github.com/jackc/pgx/v5"
)

type IRoleDao interface {
	FindAllByUserId(ctx context.Context, userId string) ([]dto.RoleDTO, error)
	FindById(ctx context.Context, roleId string) (dto.RoleDTO, error)
	FindAll(ctx context.Context, pageSize, offset int) ([]dto.RoleDTO, error)
	Save(ctx context.Context, role dto.RoleDTO) (dto.RoleDTO, error)
	Update(ctx context.Context, role dto.RoleDTO) error
	FindByName(ctx context.Context, name string) (dto.RoleDTO, error)
	SetActive(ctx context.Context, id string, active bool) error
	Count(ctx context.Context) (int, error)
}

type RoleDao struct {
	db database.DBTX
}

func NewRoleDaoInstance(db database.DBTX) IRoleDao {
	return initModules.Once(func() *RoleDao {
		return &RoleDao{db: db}
	})
}

// NewRoleDaoForTest builds a DAO without process-wide Once (tests only).
func NewRoleDaoForTest(db database.DBTX) IRoleDao {
	return &RoleDao{db: db}
}

func (r *RoleDao) FindAllByUserId(ctx context.Context, userId string) ([]dto.RoleDTO, error) {
	const q = `SELECT r.id, r.name, r.routes, r.permissions, r.active
FROM public.roles r
INNER JOIN public.users_roles ur ON ur.role_id = r.id
WHERE ur.user_id = $1
ORDER BY r.id`
	rows, err := r.db.Query(ctx, q, userId)
	if err != nil {
		return nil, fmt.Errorf("query roles by user: %w", err)
	}
	defer rows.Close()

	list := make([]dto.RoleDTO, 0)
	for rows.Next() {
		var item dto.RoleDTO
		var routes []authBaseDomain.RoutePathEnum
		if err := rows.Scan(&item.Id, &item.Name, &routes, &item.Permissions, &item.Active); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		item.Routes = routes
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate roles by user: %w", err)
	}
	return list, nil
}

func (r *RoleDao) FindById(ctx context.Context, roleId string) (dto.RoleDTO, error) {
	const q = `SELECT r.id, r.name, r.routes, r.permissions, r.active FROM public.roles r WHERE r.id = $1`
	var item dto.RoleDTO
	var routes []authBaseDomain.RoutePathEnum
	err := r.db.QueryRow(ctx, q, roleId).Scan(&item.Id, &item.Name, &routes, &item.Permissions, &item.Active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.RoleDTO{}, domain.ErrNotFound
		}
		return dto.RoleDTO{}, fmt.Errorf("find role by id: %w", err)
	}
	item.Routes = routes
	return item, nil
}

func (r *RoleDao) FindAll(ctx context.Context, pageSize, offset int) ([]dto.RoleDTO, error) {
	const q = `SELECT r.id, r.name, r.routes, r.permissions, r.active
FROM public.roles r ORDER BY r.id LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(ctx, q, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("query roles: %w", err)
	}
	defer rows.Close()

	list := make([]dto.RoleDTO, 0)
	for rows.Next() {
		var item dto.RoleDTO
		var routes []authBaseDomain.RoutePathEnum
		if err := rows.Scan(&item.Id, &item.Name, &routes, &item.Permissions, &item.Active); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		item.Routes = routes
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate roles: %w", err)
	}
	return list, nil
}

func (r *RoleDao) Save(ctx context.Context, role dto.RoleDTO) (dto.RoleDTO, error) {
	const q = `INSERT INTO public.roles (name, routes, permissions) VALUES ($1, $2, $3) RETURNING id`
	var id string
	err := r.db.QueryRow(ctx, q, role.Name, role.Routes, role.Permissions).Scan(&id)
	if err != nil {
		return dto.RoleDTO{}, fmt.Errorf("save role: %w", err)
	}
	role.Id = id
	return role, nil
}

func (r *RoleDao) FindByName(ctx context.Context, name string) (dto.RoleDTO, error) {
	const q = `SELECT r.id, r.name, r.routes, r.permissions, r.active FROM public.roles r WHERE r.name = $1`
	var item dto.RoleDTO
	var routes []authBaseDomain.RoutePathEnum
	err := r.db.QueryRow(ctx, q, name).Scan(&item.Id, &item.Name, &routes, &item.Permissions, &item.Active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.RoleDTO{}, nil
		}
		return dto.RoleDTO{}, fmt.Errorf("find role by name: %w", err)
	}
	item.Routes = routes
	return item, nil
}

func (r *RoleDao) SetActive(ctx context.Context, id string, active bool) error {
	tag, err := r.db.Exec(ctx, `UPDATE public.roles SET active = $2 WHERE id = $1`, id, active)
	if err != nil {
		return fmt.Errorf("set role active: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *RoleDao) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(id) FROM public.roles`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count roles: %w", err)
	}
	return count, nil
}

func (r *RoleDao) Update(ctx context.Context, role dto.RoleDTO) error {
	const q = `UPDATE public.roles SET name = $1, routes = $2, permissions = $3 WHERE id = $4`
	tag, err := r.db.Exec(ctx, q, role.Name, role.Routes, role.Permissions, role.Id)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
