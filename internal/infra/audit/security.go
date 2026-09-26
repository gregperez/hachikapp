package audit

import (
	"context"
	"fmt"
	"log/slog"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
)

// SecuritySink writes typed authBase events to slog. It never logs tokens,
// secrets, emails, or high-cardinality metric labels.
type SecuritySink struct {
	log *slog.Logger
}

func NewSecuritySink(log *slog.Logger) (SecuritySink, error) {
	if log == nil {
		return SecuritySink{}, fmt.Errorf("audit logger is required")
	}
	return SecuritySink{log: log}, nil
}

func (s SecuritySink) Record(_ context.Context, event authBaseCore.SecurityEvent) {
	s.log.Info("security event",
		"type", string(event.Type),
		"outcome", event.Outcome,
		"provider", event.Provider,
		"actor_id", event.ActorID,
		"family_id", event.FamilyID,
		"token_id", event.TokenID,
	)
}

// FanoutOIDC mirrors OIDC audit events onto the typed security sink.
type FanoutOIDC struct {
	Inner ports.OIDCAuditor
	Sink  authBaseCore.SecurityEventSink
}

func (f FanoutOIDC) OIDCEvent(action, provider, actorID, outcome string) {
	if f.Inner != nil {
		f.Inner.OIDCEvent(action, provider, actorID, outcome)
	}
	if f.Sink == nil {
		return
	}
	f.Sink.Record(context.Background(), authBaseCore.SecurityEvent{
		Type:     oidcSecurityType(action),
		ActorID:  actorID,
		Provider: provider,
		Outcome:  outcome,
	})
}

func oidcSecurityType(action string) authBaseCore.SecurityEventType {
	switch action {
	case "start":
		return authBaseCore.SecurityOIDCStart
	case "success":
		return authBaseCore.SecurityOIDCSuccess
	case "link":
		return authBaseCore.SecurityOIDCLink
	case "conflict":
		return authBaseCore.SecurityOIDCConflict
	default:
		return authBaseCore.SecurityOIDCFailure
	}
}

var (
	_ authBaseCore.SecurityEventSink = SecuritySink{}
	_ ports.OIDCAuditor              = FanoutOIDC{}
)
