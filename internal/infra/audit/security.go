package audit

import (
	"context"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
)

// FanoutOIDC maps OIDC audit strings onto the typed security sink once.
// Inner is only used when Sink is nil so events are not duplicated.
type FanoutOIDC struct {
	Inner ports.OIDCAuditor
	Sink  authBaseCore.SecurityEventSink
}

func (f FanoutOIDC) OIDCEvent(action, provider, actorID, outcome string) {
	if f.Sink != nil {
		f.Sink.Record(context.Background(), authBaseCore.SecurityEvent{
			Type:     oidcSecurityType(action),
			ActorID:  actorID,
			Provider: provider,
			Outcome:  outcome,
		})
		return
	}
	if f.Inner != nil {
		f.Inner.OIDCEvent(action, provider, actorID, outcome)
	}
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

var _ ports.OIDCAuditor = FanoutOIDC{}
