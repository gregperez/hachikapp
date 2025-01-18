package ports

import (
	middlewarePorts "github.com/WilsonSayago/authBase/core/ports"
	"jwt/base/internal/core/domain"
)

type UserPort interface {
	middlewarePorts.GenericPort[domain.User]
	Save(v interface{}) error
	FindAll() ([]domain.User, error)
}
