package users

import "github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto"

// UseDto represents a user dto.
// @swagger:model
// @Description UseDto represents a user dto.
type UseDto struct {
	dto.BaseDto
	Name     string   `json:"name,omitempty" binding:"required"`
	Email    string   `json:"email,omitempty" binding:"required"`
	Password string   `json:"password,omitempty" binding:"required"`
	IsAdmin  bool     `json:"isAdmin,omitempty" default:"false" example:"false"`
	Roles    []string `json:"roles,omitempty" binding:"required" example:"1sdasdsd12,2sdasdsd12"`
}
