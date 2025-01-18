package dto

type UserDTO struct {
	BaseDTO
	Email       string          `bson:"email,omitempty"`
	Password    string          `bson:"password,omitempty"`
	Permissions []PermissionDTO `bson:"permissions,omitempty"`
	Roles       []RoleDTO       `bson:"roles,omitempty"`
	IsAdmin     bool            `bson:"is_admin,omitempty"`
}
