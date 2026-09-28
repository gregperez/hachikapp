package domain

import "time"

// Session is a user-visible refresh family. It never includes token material.
type Session struct {
	FamilyID    string
	Current     bool
	IssuedAt    time.Time
	LastSeenAt  time.Time
	ExpiresAt   time.Time
	ClientLabel string
	IPPrefix    string
}
