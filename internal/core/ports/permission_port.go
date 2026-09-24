package ports

import (
	"context"

	domain "github.com/WilsonSayago/authBase/v3/core/domain"
)

type PermissionPort interface {
	FindAllByUser(ctx context.Context, id string) ([]domain.Permission, error)
}
