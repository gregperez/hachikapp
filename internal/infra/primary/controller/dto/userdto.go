package dto

type UserDto struct {
	BaseDto
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	IsAdmin  bool   `json:"isAdmin,omitempty"`
}
