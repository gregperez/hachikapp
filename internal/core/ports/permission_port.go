package ports

import (
	domain "github.com/WilsonSayago/authBase/v3/core/domain"
)

type PermissionPort interface {
	FindAllByUser(id string) ([]domain.Permission, error)
}
