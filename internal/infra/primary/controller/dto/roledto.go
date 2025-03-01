package dto

import "github.com/WilsonSayago/authBase/core/domain"

// RoleDto represents a role dto.
// @swagger:model
// @Description RoleDto represents a role dto.
type RoleDto struct {
	BaseDto
	Name        string                 `json:"name,omitempty"`
	Routes      []domain.RoutePathEnum `json:"routes,omitempty"`
	Permissions []PermissionDto        `json:"permissions,omitempty"`
}
