package ports

import (
	domain "github.com/WilsonSayago/middleware/core/domains"
)

type PermissionPort interface {
	FindAllByUser(id string) ([]domain.Permission, error)
}
