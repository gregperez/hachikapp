package dao

import "jwt/base/internal/infra/secondary/repositories/dto"

type IUserDao interface {
	Save(user dto.UserDTO) error
	FindById(id string) (dto.UserDTO, error)
	FindByUserId(userId string) ([]interface{}, error)
	FindAll() ([]dto.UserDTO, error)
	FindByEmail(email string) (dto.UserDTO, error)
	Update(user dto.UserDTO) error
	ChangeStatus(id string) error
}
