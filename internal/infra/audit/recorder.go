package audit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/requestctx"
)

// Recorder writes one slog line and one sanitized row per event.
// Insert failures are logged and never returned to the caller.
type Recorder struct {
	log   *slog.Logger
	store ports.AuditEventPort
	now   func() time.Time
}

func NewRecorder(log *slog.Logger, store ports.AuditEventPort) (Recorder, error) {
	if log == nil {
		return Recorder{}, fmt.Errorf("audit logger is required")
	}
	return Recorder{log: log, store: store, now: time.Now}, nil
}

var (
	_ authBaseCore.SecurityEventSink = Recorder{}
	_ ports.PrivilegeAuditor         = Recorder{}
	_ ports.SettingsAuditor          = Recorder{}
	_ ports.OIDCAuditor              = Recorder{}
)

func (r Recorder) Record(ctx context.Context, event authBaseCore.SecurityEvent) {
	action := string(event.Type)
	if !domain.KnownAuditAction(action) {
		r.log.Warn("security event ignored", "type", action, "outcome", event.Outcome)
		return
	}
	outcome := event.Outcome
	if !domain.KnownAuditOutcome(outcome) {
		outcome = domain.AuditOutcomeFailure
		if event.Outcome == "success" {
			outcome = domain.AuditOutcomeSuccess
		}
	}
	r.log.Info("security event",
		"type", action,
		"outcome", outcome,
		"provider", event.Provider,
		"actor_id", domain.NormalizeAuditActor(event.ActorID),
		"family_id", event.FamilyID,
		"token_id", event.TokenID,
	)
	subjectType, subjectID := securitySubject(event)
	r.persist(ctx, domain.AuditEvent{
		OccurredAt:  r.now().UTC(),
		Category:    domain.CategoryForAuditAction(action),
		Action:      action,
		Outcome:     outcome,
		ActorID:     event.ActorID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		RequestID:   requestctx.ID(ctx),
		Metadata: domain.SanitizeAuditMetadata(map[string]any{
			"provider": event.Provider,
			"familyId": event.FamilyID,
			"tokenId":  event.TokenID,
		}),
	})
}

func (r Recorder) SettingsChange(ctx context.Context, actorID string, fields []string) {
	r.log.Info("privilege change",
		"action", "update_settings",
		"actor_id", domain.NormalizeAuditActor(actorID),
		"subject_id", "app",
		"fields", fields,
	)
	r.persist(ctx, domain.AuditEvent{
		OccurredAt:  r.now().UTC(),
		Category:    domain.AuditCategoryPrivilege,
		Action:      "update_settings",
		Outcome:     domain.AuditOutcomeSuccess,
		ActorID:     actorID,
		SubjectType: domain.AuditSubjectSettings,
		SubjectID:   "app",
		RequestID:   requestctx.ID(ctx),
		Metadata: domain.SanitizeAuditMetadata(map[string]any{
			"fields": fields,
		}),
	})
}

func (r Recorder) PrivilegeChange(ctx context.Context, action, actorID, subjectID string, roleIDs []string, isAdmin *bool) {
	if action == "" {
		action = "privilege_change"
	}
	if !domain.KnownAuditAction(action) {
		r.log.Warn("privilege event ignored", "action", action)
		return
	}
	attrs := []any{
		"action", action,
		"actor_id", domain.NormalizeAuditActor(actorID),
		"subject_id", subjectID,
	}
	meta := map[string]any{}
	if len(roleIDs) > 0 {
		attrs = append(attrs, "role_ids", roleIDs)
		meta["roleIds"] = roleIDs
	}
	if isAdmin != nil {
		attrs = append(attrs, "is_admin", *isAdmin)
		meta["isAdmin"] = *isAdmin
	}
	r.log.Info("privilege change", attrs...)
	r.persist(ctx, domain.AuditEvent{
		OccurredAt:  r.now().UTC(),
		Category:    domain.AuditCategoryPrivilege,
		Action:      action,
		Outcome:     domain.AuditOutcomeSuccess,
		ActorID:     actorID,
		SubjectType: domain.AuditSubjectUser,
		SubjectID:   subjectID,
		RequestID:   requestctx.ID(ctx),
		Metadata:    domain.SanitizeAuditMetadata(meta),
	})
}

func (r Recorder) OIDCEvent(action, provider, actorID, outcome string) {
	r.Record(context.Background(), authBaseCore.SecurityEvent{
		Type:     oidcSecurityType(action),
		ActorID:  actorID,
		Provider: provider,
		Outcome:  outcome,
	})
}

func (r Recorder) persist(ctx context.Context, event domain.AuditEvent) {
	if r.store == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := r.store.Insert(ctx, event); err != nil {
		r.log.Error("audit persist failed", "err", err)
	}
}

func securitySubject(event authBaseCore.SecurityEvent) (string, string) {
	switch event.Type {
	case authBaseCore.SecurityRefreshSuccess, authBaseCore.SecurityRefreshReplay, authBaseCore.SecurityFamilyRevoke:
		return domain.AuditSubjectSession, event.FamilyID
	case authBaseCore.SecurityOIDCStart, authBaseCore.SecurityOIDCSuccess, authBaseCore.SecurityOIDCFailure,
		authBaseCore.SecurityOIDCLink, authBaseCore.SecurityOIDCConflict:
		return domain.AuditSubjectOIDC, event.ActorID
	case authBaseCore.SecurityTokenReject, authBaseCore.SecurityKeyRotation:
		return domain.AuditSubjectSystem, ""
	default:
		return domain.AuditSubjectUser, event.ActorID
	}
}

// SecuritySink is kept for tests that only assert slog output.
type SecuritySink = Recorder

func NewSecuritySink(log *slog.Logger) (Recorder, error) {
	return NewRecorder(log, nil)
}

// Logger is the historical privilege auditor name used by bootstrap tests.
type Logger = Recorder

func NewLogger(log *slog.Logger) (Recorder, error) {
	return NewRecorder(log, nil)
}
