package ports

import (
	"context"

	domain "github.com/WilsonSayago/authBase/v4/core/domain"
)

type PermissionPort interface {
	FindAllByUser(ctx context.Context, id string) ([]domain.Permission, error)
}
