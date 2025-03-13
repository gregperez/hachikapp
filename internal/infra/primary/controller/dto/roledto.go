package dto

import "github.com/WilsonSayago/authBase/core/domain"

// RoleDto represents a role with its associated permissions and routes
// @swagger:model
// @Description RoleDto contains the information about a role including its name, routes, and permissions
type RoleDto struct {
	BaseDto
	Name        string                 `json:"name,omitempty"`
	Routes      []domain.RoutePathEnum `json:"routes,omitempty"`
	Permissions []PermissionDto        `json:"permissions,omitempty"`
}
