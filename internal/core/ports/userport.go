package ports

import (
	middlewarePorts "github.com/WilsonSayago/authBase/core/port"
	"jwt/base/internal/core/domain"
)

type UserPort interface {
	middlewarePorts.GenericPort[domain.User]
	Save(u domain.User) error
	FindAll() ([]domain.User, error)
	FindById(id string) (domain.User, error)
	FindByEmail(email string) (domain.User, error)
	Update(u domain.User) error
	ChangeStatus(id string) error
}
