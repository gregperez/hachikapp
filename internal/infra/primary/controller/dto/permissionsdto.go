package dto

// PermissionDto represents the permissions for a specific entity.
// @Description PermissionDto defines the CRUD permissions for a specific entity
type PermissionDto struct {
	Entity string `json:"entity,omitempty"`
	Create bool   `json:"create,omitempty"`
	Read   bool   `json:"read,omitempty"`
	Update bool   `json:"update,omitempty"`
	Delete bool   `json:"delete,omitempty"`
}
