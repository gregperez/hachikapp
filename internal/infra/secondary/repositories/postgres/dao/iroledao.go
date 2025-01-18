package dao

import "jwt/base/internal/infra/secondary/repositories/dto"

type IRoleDao interface {
	FindAllByUserId(userId string) ([]dto.RoleDTO, error)
}
