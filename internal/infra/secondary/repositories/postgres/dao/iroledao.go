package dao

import "jwt/base/internal/infra/secondary/repositories/dto"

type IRoleDao interface {
	FindAllByUserId(userId string) ([]dto.RoleDTO, error)
	FindById(userId string) (dto.RoleDTO, error)
	FindAll() ([]dto.RoleDTO, error)
	Save(role dto.RoleDTO) (dto.RoleDTO, error)
	FindByName(name string) (dto.RoleDTO, error)
}
