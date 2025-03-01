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
	Password    string                     `json:"password,omitempty"`
	IsAdmin     bool                       `json:"isAdmin,omitempty"`
	Permissions []roles.PermissionResponse `json:"permissions,omitempty"`
}

func (userResponse *UserResponse) MapDomainToResponse(user domain.User) {
	userResponse.BaseDto.DomainToEntity(user.Base)
	userResponse.Name = user.GetName()
	userResponse.Email = user.GetEmail()
	userResponse.Password = user.GetPassword()
	userResponse.IsAdmin = user.GetIsAdmin()
	permissions := make([]roles.PermissionResponse, 0)
	for _, permission := range user.GetPermissions() {
		permissionResponse := roles.PermissionResponse{}
		permissionResponse.MapToEntity(permission)
		permissions = append(permissions, permissionResponse)
	}
	userResponse.Permissions = permissions
}
