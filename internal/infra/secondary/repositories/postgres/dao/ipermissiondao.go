package dao

import "jwt/base/internal/infra/secondary/repositories/dto"

type IPermissionDao interface {
	FindAllByUserId(userId string) ([]dto.PermissionDTO, error)
}
