package audit

import (
	"github.com/WilsonSayago/base-golang/internal/core/ports"
	log "github.com/sirupsen/logrus"
)

// Logger implements ports.PrivilegeAuditor with structured logs (no secrets).
type Logger struct{}

func NewLogger() ports.PrivilegeAuditor {
	return Logger{}
}

func (Logger) PrivilegeChange(action, actorID, subjectID string, roleIDs []string, isAdmin *bool) {
	fields := log.Fields{
		"action":     action,
		"actor_id":   actorID,
		"subject_id": subjectID,
	}
	if len(roleIDs) > 0 {
		fields["role_ids"] = roleIDs
	}
	if isAdmin != nil {
		fields["is_admin"] = *isAdmin
	}
	log.WithFields(fields).Info("privilege change")
}
