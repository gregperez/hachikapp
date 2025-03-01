package users

// UserUpdateDto represents a user update dto.
// @swagger:model
// @Description UserUpdateDto represents a user update dto.
type UserUpdateDto struct {
	Name    string `json:"name,omitempty" binding:"required"`
	Email   string `json:"email,omitempty" binding:"required"`
	IsAdmin bool   `json:"isAdmin,omitempty" binding:"required"`
}
