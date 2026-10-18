package users

import "github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto"

// UseDto is retained for generated swagger compatibility.
// Prefer CreateUserRequest — privileged fields are intentionally absent.
//
// Deprecated: use CreateUserRequest.
type UseDto struct {
	dto.BaseDto
	Name     string `json:"name,omitempty" binding:"required"`
	Email    string `json:"email,omitempty" binding:"required,email"`
	Password string `json:"password,omitempty" binding:"required"`
}
