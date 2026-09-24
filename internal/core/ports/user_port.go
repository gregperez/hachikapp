package ports

import (
	"github.com/WilsonSayago/authBase/v3/core/port"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
)

// UserPort is the application persistence port for users.
// It also satisfies authBase UserReader and CredentialReader.
type UserPort interface {
	port.UserReader[domain.User]
	port.CredentialReader
	Save(u domain.User) error
	FindAll(pageSize, offset int) ([]domain.User, int, error)
	FindById(id string) (domain.User, error)
	FindByEmail(email string) (domain.User, error)
	FindFullById(id string) (domain.User, error)
	Update(u domain.User) error
	ChangeStatus(id string) error
}
