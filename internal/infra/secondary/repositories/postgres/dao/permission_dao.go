package dao

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/dto"
	"github.com/WilsonSayago/initModules/v2"
)

type IPermissionDao interface {
	FindAllByUserId(ctx context.Context, userId string) ([]dto.PermissionDTO, error)
}

type PermissionDao struct {
	db database.DBTX
}

func NewPermissionDaoInstance(db database.DBTX) IPermissionDao {
	return initModules.Once(func() *PermissionDao {
		return &PermissionDao{db: db}
	})
}

func (c *PermissionDao) FindAllByUserId(ctx context.Context, id string) ([]dto.PermissionDTO, error) {
	const q = `SELECT p.entity, p.create, p.read, p.update, p.delete
FROM public.permissions p
INNER JOIN public.users_permissions up ON up.permission_id = p.id
WHERE up.user_id = $1
ORDER BY p.entity`
	rows, err := c.db.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("query permissions: %w", err)
	}
	defer rows.Close()

	list := make([]dto.PermissionDTO, 0)
	for rows.Next() {
		var entity string
		var create, read, update, deleteattr sql.NullBool
		if err := rows.Scan(&entity, &create, &read, &update, &deleteattr); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		list = append(list, dto.PermissionDTO{
			Entity: entity,
			Create: create.Bool,
			Read:   read.Bool,
			Update: update.Bool,
			Delete: deleteattr.Bool,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate permissions: %w", err)
	}
	return list, nil
}
