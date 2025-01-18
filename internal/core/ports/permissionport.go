package ports

import (
	domain "github.com/WilsonSayago/authBase/core/domains"
)

type PermissionPort interface {
	FindAllByUser(id string) ([]domain.Permission, error)
}
