package users

// CreateUserRequest creates a non-privileged user. Roles and isAdmin are assigned
// only via the privileges endpoints.
type CreateUserRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=12"`
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
type SetAdminFlagRequest struct {
	IsAdmin bool `json:"isAdmin"`
}

// ChangePasswordRequest changes the caller's password after verifying the current one.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=12"`
}

// SetActiveRequest sets the desired active flag (idempotent).
type SetActiveRequest struct {
	Active bool `json:"active"`
}
