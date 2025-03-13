package dto

// PermissionDto represents the permissions for a specific entity
// @swagger:model
// @Description PermissionDto defines the CRUD permissions for a specific entity
type PermissionDto struct {
	Entity string `bson:"entity,omitempty"`
	Create bool   `bson:"create,omitempty"`
	Read   bool   `bson:"read,omitempty"`
	Update bool   `bson:"update,omitempty"`
	Delete bool   `bson:"delete,omitempty"`
}
