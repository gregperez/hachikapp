package ports

import (
	middlewarePorts "github.com/WilsonSayago/middleware/core/ports"
	"jwt/base/internal/core/domain"
)

type RepositoryPort interface {
	middlewarePorts.GenericPort[domain.User]
	Save(v interface{}) error
}
