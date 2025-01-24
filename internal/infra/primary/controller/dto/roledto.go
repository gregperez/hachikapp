package dto

import "github.com/WilsonSayago/authBase/core/domain"

type RoleDto struct {
	Id          string                 `bson:"id,omitempty"`
	Name        string                 `bson:"name,omitempty"`
	Routes      []domain.RoutePathEnum `bson:"routes,omitempty"`
	Permissions []PermissionDto        `bson:"permissions,omitempty"`
}
