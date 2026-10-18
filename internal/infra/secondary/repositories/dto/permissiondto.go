package dto

type PermissionDTO struct {
	Entity string `json:"entity,omitempty"`
	Create bool   `json:"create,omitempty"`
	Read   bool   `json:"read,omitempty"`
	Update bool   `json:"update,omitempty"`
	Delete bool   `json:"delete,omitempty"`
}
