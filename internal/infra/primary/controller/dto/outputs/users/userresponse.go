package users

import (
	"jwt/base/internal/core/domain"
	"jwt/base/internal/infra/primary/controller/dto"
	"jwt/base/internal/infra/primary/controller/dto/outputs/roles"
)

// UserResponse represents a user response.
// @swagger:model
// @Description UserResponse represents a user response.
type UserResponse struct {
	dto.BaseDto
	Name        string                     `json:"name,omitempty"`
	Email       string                     `json:"email,omitempty"`
	IsAdmin     bool                       `json:"isAdmin,omitempty"`
	Permissions []roles.PermissionResponse `json:"permissions,omitempty"`
	Roles       []roles.RoleResponse       `json:"roles,omitempty"`
}

func (userResponse *UserResponse) MapDomainToResponse(user domain.User) {
	userResponse.BaseDto.DomainToEntity(user.Base)
	userResponse.Name = user.GetName()
	userResponse.Email = user.GetEmail()
	userResponse.IsAdmin = user.GetIsAdmin()
	permissions := make([]roles.PermissionResponse, 0)
	for _, permission := range user.GetPermissions() {
		permissionResponse := roles.PermissionResponse{}
		permissionResponse.MapToEntity(permission)
		permissions = append(permissions, permissionResponse)
	}
	userResponse.Permissions = permissions

	rolesList := make([]roles.RoleResponse, 0)
	for _, role := range user.GetRole() {
		roleResponse := roles.MapDomainToResponse(role)
		rolesList = append(rolesList, roleResponse)
	}
	userResponse.Roles = rolesList
}

func MapDomainToUserResponse(user domain.User) UserResponse {
	var response UserResponse
	response.MapDomainToResponse(user)
	return response
}
