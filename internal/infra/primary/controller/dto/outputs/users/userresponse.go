package users

import "jwt/base/internal/infra/primary/controller/dto"

type UserResponse struct {
	dto.BaseDto
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	IsAdmin  bool   `json:"isAdmin,omitempty"`
}
