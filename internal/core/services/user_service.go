package services

import (
	"context"

	"github.com/WilsonSayago/base-golang/internal/core"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
	initModules "github.com/WilsonSayago/initModules/v2"
)

type UserService struct {
	userRepository ports.UserPort
}

func GetUserServiceInstance(userRepository ports.UserPort) core.UserUseCase {
	return initModules.Once(func() *UserService {
		return &UserService{userRepository: userRepository}
	})
}

func (s *UserService) FindByID(ctx context.Context, id string) (domain.User, error) {
	return s.userRepository.FindByID(ctx, id)
}

func (s *UserService) FindAll(pageSize, offset int) ([]domain.User, int, error) {
	return s.userRepository.FindAll(pageSize, offset)
}

func (s *UserService) FindByEmail(email string) (domain.User, error) {
	return s.userRepository.FindByEmail(email)
}

func (s *UserService) FindFullById(id string) (domain.User, error) {
	return s.userRepository.FindFullById(id)
}

func (s *UserService) Create(user domain.User) error {
	return s.userRepository.Save(user)
}

func (s *UserService) FindById(id string) (domain.User, error) {
	return s.userRepository.FindById(id)
}

func (s *UserService) Update(user domain.User) error {
	return s.userRepository.Update(user)
}

func (s *UserService) ChangeStatus(id string) error {
	return s.userRepository.ChangeStatus(id)
}
