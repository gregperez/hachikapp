package domain

import "errors"

var (
	// ErrForbidden is returned when an authenticated caller lacks permission
	// for a privileged mutation (maps to HTTP 403).
	ErrForbidden = errors.New("forbidden")

	// ErrLastAdmin protects against removing the final administrator.
	ErrLastAdmin = errors.New("cannot disable or demote the last administrator")

	// ErrWeakPassword is returned when a password does not meet policy.
	ErrWeakPassword = errors.New("password does not meet the current policy")

	// ErrAdminExists is returned by bootstrap when an administrator already exists.
	ErrAdminExists = errors.New("administrator already exists")

	// ErrNotFound is returned when a mutation or lookup targets a missing row.
	ErrNotFound = errors.New("not found")

	// ErrConflict is returned when a unique/natural-key constraint would be violated
	// (maps to HTTP 409).
	ErrConflict = errors.New("conflict")
)
