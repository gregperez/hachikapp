package dto

type UserDto struct {
	Email   string `bson:"email,omitempty"`
	IsAdmin bool   `bson:"isAdmin,omitempty"`
}
