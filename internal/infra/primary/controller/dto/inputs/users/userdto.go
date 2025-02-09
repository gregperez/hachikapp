package users

import "jwt/base/internal/infra/primary/controller/dto"

type UseDto struct {
	dto.BaseDto
	Name     string `json:"name,omitempty" binding:"required"`
	Email    string `json:"email,omitempty" binding:"required"`
	Password string `json:"password,omitempty" binding:"required"`
	IsAdmin  bool   `json:"isAdmin,omitempty" binding:"required"`
}
