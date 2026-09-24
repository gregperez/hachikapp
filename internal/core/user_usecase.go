package core

import (
	"github.com/WilsonSayago/authBase/v3/core"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
)

type UserUseCase interface {
	core.GenericUseCase[domain.User]
	FindAll(pageSize, offset int) ([]domain.User, int, error)
	FindByEmail(email string) (domain.User, error)
	Create(user domain.User) error
	FindById(id string) (domain.User, error)
	FindFullById(id string) (domain.User, error)
	Update(user domain.User) error
	ChangeStatus(id string) error
}
