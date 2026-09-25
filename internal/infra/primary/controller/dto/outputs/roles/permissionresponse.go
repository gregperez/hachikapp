package roles

import "github.com/WilsonSayago/authBase/v4/core/domain"

// PermissionResponse represents a permission response.
// @swagger:model
// @Description PermissionResponse represents a permission response.
type PermissionResponse struct {
	Entity string `json:"entity,omitempty"`
	Create bool   `json:"create,omitempty"`
	Read   bool   `json:"read,omitempty"`
	Update bool   `json:"update,omitempty"`
	Delete bool   `json:"delete,omitempty"`
}

func (permissionResponse *PermissionResponse) MapToEntity(permission domain.Permission) {
	permissionResponse.Entity = permission.Entity
	permissionResponse.Create = permission.Create
	permissionResponse.Read = permission.Read
	permissionResponse.Update = permission.Update
	permissionResponse.Delete = permission.Delete
}
