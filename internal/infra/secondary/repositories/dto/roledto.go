package dto

import domain "github.com/WilsonSayago/authBase/v3/core/domain"

type RoleDTO struct {
	BaseDTO
	Name        string `bson:"entity,omitempty"`
	Routes      []domain.RoutePathEnum
	Permissions []PermissionDTO
}
