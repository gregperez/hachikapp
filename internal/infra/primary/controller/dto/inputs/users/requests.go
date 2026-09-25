package users

// CreateUserRequest creates a non-privileged user. Roles and isAdmin are assigned
// only via the privileges endpoints.
type CreateUserRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UpdateOwnProfileRequest allows a user to edit their own non-privileged fields.
// Email, isAdmin, roles, active and password are intentionally absent.
type UpdateOwnProfileRequest struct {
	Name string `json:"name" binding:"required"`
}

// AdminUpdateUserRequest updates identity fields without privileges or password.
type AdminUpdateUserRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

// AssignUserRolesRequest replaces the subject's role set.
type AssignUserRolesRequest struct {
	RoleIDs []string `json:"roleIds" binding:"required,min=1"`
}

// SetAdminFlagRequest toggles the legacy is_admin flag via an explicit flow.
// IsAdmin is a pointer so omitted fields fail validation instead of defaulting to false.
type SetAdminFlagRequest struct {
	IsAdmin *bool `json:"isAdmin" binding:"required"`
}

// ChangePasswordRequest changes the caller's password after verifying the current one.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required"`
}

// SetActiveRequest sets the desired active flag (idempotent).
// Active is a pointer so omitted fields fail validation instead of defaulting to false.
type SetActiveRequest struct {
	Active *bool `json:"active" binding:"required"`
}
