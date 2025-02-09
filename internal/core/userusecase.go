package core

import (
	"github.com/WilsonSayago/authBase/core"
	"jwt/base/internal/core/domain"
)

type UserUseCase interface {
	core.GenericUseCase[domain.User]
	FindAll() ([]domain.User, error)
	FindByEmail(email string) (domain.User, error)
	Create(user domain.User) error
	FindById(id string) (domain.User, error)
	Update(user domain.User) error
	ChangeStatus(id string) error
}
