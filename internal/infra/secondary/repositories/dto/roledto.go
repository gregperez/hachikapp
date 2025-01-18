package dto

import domain "github.com/WilsonSayago/authBase/core/domains"

type RoleDTO struct {
	BaseDTO
	Name        string `bson:"entity,omitempty"`
	Routes      []domain.RoutePathEnum
	Permissions []PermissionDTO
}
