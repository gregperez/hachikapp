package dao

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/dto"
)

type IPermissionDao interface {
	FindAllByUserId(ctx context.Context, userId string) ([]dto.PermissionDTO, error)
}

type PermissionDao struct {
	db database.DBTX
}

func NewPermissionDao(db database.DBTX) IPermissionDao {
	return &PermissionDao{db: db}
}

// NewPermissionDaoInstance is an alias for NewPermissionDao.
//
// Deprecated: use NewPermissionDao.
func NewPermissionDaoInstance(db database.DBTX) IPermissionDao {
	return NewPermissionDao(db)
}

// FindAllByUserId aggregates permissions from the JSONB on roles linked via users_roles.
// There is no separate permissions / users_permissions table in the migration baseline.
func (c *PermissionDao) FindAllByUserId(ctx context.Context, id string) ([]dto.PermissionDTO, error) {
	const q = `
SELECT r.permissions
FROM public.roles r
INNER JOIN public.users_roles ur ON ur.role_id = r.id
WHERE ur.user_id = $1
  AND COALESCE(r.active, true) = true
ORDER BY r.name`
	rows, err := c.db.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("query role permissions: %w", err)
	}
	defer rows.Close()

	byEntity := make(map[string]dto.PermissionDTO)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan role permissions: %w", err)
		}
		if len(raw) == 0 {
			continue
		}
		var perms []dto.PermissionDTO
		if err := json.Unmarshal(raw, &perms); err != nil {
			return nil, fmt.Errorf("decode role permissions: %w", err)
		}
		for _, p := range perms {
			cur := byEntity[p.Entity]
			cur.Entity = p.Entity
			cur.Create = cur.Create || p.Create
			cur.Read = cur.Read || p.Read
			cur.Update = cur.Update || p.Update
			cur.Delete = cur.Delete || p.Delete
			byEntity[p.Entity] = cur
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate role permissions: %w", err)
	}
	list := make([]dto.PermissionDTO, 0, len(byEntity))
	for _, p := range byEntity {
		list = append(list, p)
	}
	return list, nil
}
