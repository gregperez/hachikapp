package core

import (
	"github.com/WilsonSayago/authBase/core"
	"jwt/base/internal/core/domain"
)

type IUserUseCase interface {
	core.GenericUseCase[domain.User]
	FindAll() ([]domain.User, error)
	FindByEmail(email string) (domain.User, error)
	Create(user domain.User) error
}
