package ports

// PrivilegeAuditor records privileged mutations without secrets or PII.
type PrivilegeAuditor interface {
	PrivilegeChange(action, actorID, subjectID string, roleIDs []string, isAdmin *bool)
}

// NoopAuditor discards audit events (tests).
type NoopAuditor struct{}

func (NoopAuditor) PrivilegeChange(string, string, string, []string, *bool) {}
