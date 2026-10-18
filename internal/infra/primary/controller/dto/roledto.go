package dto

import "github.com/WilsonSayago/authBase/v4/core/domain"

// RoleDto is the request body for creating/updating a role.
// @Description RoleDto contains the information about a role including its name, routes, and permissions
type RoleDto struct {
	Name        string                 `json:"name" binding:"required"`
	Routes      []domain.RoutePathEnum `json:"routes"`
	Permissions []PermissionDto        `json:"permissions"`
}
