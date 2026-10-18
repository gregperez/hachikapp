package dto

type UserDTO struct {
	BaseDTO
	Name        string          `json:"name,omitempty"`
	Email       string          `json:"email,omitempty"`
	Password    string          `json:"password,omitempty"`
	Permissions []PermissionDTO `json:"permissions,omitempty"`
	Roles       []RoleDTO       `json:"roles,omitempty"`
	IsAdmin     bool            `json:"isAdmin,omitempty"`
}
