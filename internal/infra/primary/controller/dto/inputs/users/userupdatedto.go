package users

// UserUpdateDto is retained for swagger compatibility.
// Prefer UpdateOwnProfileRequest or AdminUpdateUserRequest.
//
// Deprecated: privileged fields removed; use the dedicated request DTOs.
type UserUpdateDto struct {
	Name  string `json:"name,omitempty" binding:"required"`
	Email string `json:"email,omitempty" binding:"required,email"`
}
