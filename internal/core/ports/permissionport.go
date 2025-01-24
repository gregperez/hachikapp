package ports

import (
	domain "github.com/WilsonSayago/authBase/core/domain"
)

type PermissionPort interface {
	FindAllByUser(id string) ([]domain.Permission, error)
}
