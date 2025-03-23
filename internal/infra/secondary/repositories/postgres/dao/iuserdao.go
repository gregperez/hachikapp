package dao

import (
	"jwt/base/internal/infra/config/database"
	"jwt/base/internal/infra/secondary/repositories/dto"
)

type IUserDao interface {
	// Operaciones estándar sin transacción
	Save(user *dto.UserDTO) error
	FindById(id string) (dto.UserDTO, error)
	FindByUserId(userId string) ([]interface{}, error)
	FindAll(page, offset int) ([]dto.UserDTO, error)
	FindByEmail(email string) (dto.UserDTO, error)
	Update(user dto.UserDTO) error
	ChangeStatus(id string) error
	Count() (int, error)

	// Operaciones con transacciones
	SaveWithTx(tx database.Transaction, user *dto.UserDTO) error
	UpdateWithTx(tx database.Transaction, user dto.UserDTO) error
	ChangeStatusWithTx(tx database.Transaction, id string) error

	// Gestión de transacciones
	BeginTx() (database.Transaction, error)
}
