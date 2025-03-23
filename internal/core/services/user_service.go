package services

import (
	"github.com/WilsonSayago/initModules"
	"jwt/base/internal/core"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/core/ports"
)

type UserService struct {
	userRepository ports.UserPort
}

func GetUserServiceInstance(userRepository ports.UserPort) core.UserUseCase {
	instance := initModules.GetInstance("UserService", func() interface{} {
		return &UserService{
			userRepository: userRepository,
		}
	})
	return instance.(*UserService)
}

func (s *UserService) FindAll(pageSize, offset int) ([]domain.User, int, error) {
	return s.userRepository.FindAll(pageSize, offset)
}

func (s *UserService) FindByEmail(email string) (domain.User, error) {
	return s.userRepository.FindByEmail(email)
}

func (s *UserService) FindFullById(id string) (domain.User, error) {
	user, err := s.userRepository.FindFullById(id)
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
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
