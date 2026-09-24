package dto

import domain "github.com/WilsonSayago/authBase/v3/core/domain"

type RoleDTO struct {
	BaseDTO
	Name        string `json:"name,omitempty"`
	Routes      []domain.RoutePathEnum
	Permissions []PermissionDTO
}
