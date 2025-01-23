package dto

type UserDto struct {
	Name     string `bson:"name,omitempty"`
	Email    string `bson:"email,omitempty"`
	Password string `bson:"password,omitempty"`
	IsAdmin  bool   `bson:"isAdmin,omitempty"`
}
