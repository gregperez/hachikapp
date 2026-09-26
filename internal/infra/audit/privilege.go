package audit

import (
	"fmt"
	"log/slog"

	"github.com/WilsonSayago/base-golang/internal/core/ports"
)

// Logger implements ports.PrivilegeAuditor with structured logs (no secrets).
type Logger struct {
	log *slog.Logger
}

func NewLogger(log *slog.Logger) (Logger, error) {
	if log == nil {
		return Logger{}, fmt.Errorf("audit logger is required")
	}
	return Logger{log: log}, nil
}

var (
	_ ports.PrivilegeAuditor = Logger{}
	_ ports.OIDCAuditor      = Logger{}
)

func (l Logger) OIDCEvent(action, provider, actorID, outcome string) {
	l.log.Info("oidc event",
		"action", action,
		"provider", provider,
		"actor_id", actorID,
		"outcome", outcome,
	)
}

func (l Logger) PrivilegeChange(action, actorID, subjectID string, roleIDs []string, isAdmin *bool) {
	attrs := []any{
		"action", action,
		"actor_id", actorID,
		"subject_id", subjectID,
	}
	if len(roleIDs) > 0 {
		attrs = append(attrs, "role_ids", roleIDs)
	}
	if isAdmin != nil {
		attrs = append(attrs, "is_admin", *isAdmin)
	}
	l.log.Info("privilege change", attrs...)
}
