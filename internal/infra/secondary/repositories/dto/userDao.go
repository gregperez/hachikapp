package dto

import (
	"jwt/base/internal/core/domain"
)

type UserDTO struct {
	Id          string          `bson:"id,omitempty"`
	Email       string          `bson:"email,omitempty"`
	Password    string          `bson:"password,omitempty"`
	Permissions []PermissionDTO `bson:"permissions,omitempty"`
	IsAdmin     bool            `bson:"is_admin,omitempty"`
}

type PermissionDTO struct {
	Entity domain.EntityEnum `bson:"entity,omitempty"`
	Create bool              `bson:"create,omitempty"`
	Read   bool              `bson:"read,omitempty"`
	Update bool              `bson:"update,omitempty"`
	Delete bool              `bson:"delete,omitempty"`
}
