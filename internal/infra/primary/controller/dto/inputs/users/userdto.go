package users

import "jwt/base/internal/infra/primary/controller/dto"

// UseDto represents a user dto.
// @swagger:model
// @Description UseDto represents a user dto.
type UseDto struct {
	dto.BaseDto
	Name     string `json:"name,omitempty" binding:"required"`
	Email    string `json:"email,omitempty" binding:"required"`
	Password string `json:"password,omitempty" binding:"required"`
	IsAdmin  bool   `json:"isAdmin,omitempty" binding:"required"`
}
