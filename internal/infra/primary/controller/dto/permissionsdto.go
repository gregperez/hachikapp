package dto

type PermissionDto struct {
	Entity string `bson:"entity,omitempty"`
	Create bool   `bson:"create,omitempty"`
	Read   bool   `bson:"read,omitempty"`
	Update bool   `bson:"update,omitempty"`
	Delete bool   `bson:"delete,omitempty"`
}
