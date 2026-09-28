package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	AuditCategorySecurity  = "security"
	AuditCategoryPrivilege = "privilege"

	AuditOutcomeSuccess = "success"
	AuditOutcomeFailure = "failure"

	AuditActorSystem = "system"

	AuditSubjectUser    = "user"
	AuditSubjectSession = "session"
	AuditSubjectOIDC    = "oidc"
	AuditSubjectSystem  = "system"
)

const (
	maxAuditActionLen   = 64
	maxAuditActorLen    = 64
	maxAuditMetadataLen = 80
	maxAuditRoleIDs     = 20
)

// AuditEvent is a sanitized, append-only administrative record.
type AuditEvent struct {
	ID          string
	OccurredAt  time.Time
	Category    string
	Action      string
	Outcome     string
	ActorID     string
	SubjectType string
	SubjectID   string
	RequestID   string
	Metadata    map[string]any
}

// AuditListFilter is the closed set of list predicates. Values are validated
// before they reach SQL; callers never pass raw expressions.
type AuditListFilter struct {
	Action    string
	Outcome   string
	ActorID   string
	SubjectID string
	From      time.Time
	To        time.Time
}

var allowedAuditActions = map[string]string{
	"login_success":   AuditCategorySecurity,
	"login_failure":   AuditCategorySecurity,
	"refresh_success": AuditCategorySecurity,
	"refresh_replay":  AuditCategorySecurity,
	"family_revoke":   AuditCategorySecurity,
	"user_revoke":     AuditCategorySecurity,
	"token_reject":    AuditCategorySecurity,
	"key_rotation":    AuditCategorySecurity,
	"oidc_start":      AuditCategorySecurity,
	"oidc_success":    AuditCategorySecurity,
	"oidc_failure":    AuditCategorySecurity,
	"oidc_link":       AuditCategorySecurity,
	"oidc_conflict":   AuditCategorySecurity,
	"assign_roles":    AuditCategoryPrivilege,
	"set_admin_flag":  AuditCategoryPrivilege,
	"set_active":      AuditCategoryPrivilege,
}

var allowedAuditMetadataKeys = map[string]struct{}{
	"provider": {},
	"roleIds":  {},
	"isAdmin":  {},
	"familyId": {},
	"tokenId":  {},
}

var forbiddenAuditSubstrings = []string{
	"password", "passwd", "secret", "token", "jwt", "bearer", "cookie",
	"authorization", "dsn", "postgres://", "prompt", "email", "@",
}

// KnownAuditAction reports whether action is in the closed taxonomy.
func KnownAuditAction(action string) bool {
	_, ok := allowedAuditActions[action]
	return ok
}

// CategoryForAuditAction returns the category for a known action.
func CategoryForAuditAction(action string) string {
	return allowedAuditActions[action]
}

// KnownAuditOutcome reports whether outcome is success or failure.
func KnownAuditOutcome(outcome string) bool {
	return outcome == AuditOutcomeSuccess || outcome == AuditOutcomeFailure
}

// NormalizeAuditActor returns system when actor is empty.
func NormalizeAuditActor(actorID string) string {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return AuditActorSystem
	}
	if utf8.RuneCountInString(actorID) > maxAuditActorLen {
		return string([]rune(actorID)[:maxAuditActorLen])
	}
	return actorID
}

// SanitizeAuditMetadata drops unknown keys and values that look like secrets or PII.
func SanitizeAuditMetadata(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		if _, ok := allowedAuditMetadataKeys[key]; !ok {
			continue
		}
		switch key {
		case "isAdmin":
			if b, ok := value.(bool); ok {
				out[key] = b
			}
		case "roleIds":
			if ids := sanitizeStringSlice(value); len(ids) > 0 {
				out[key] = ids
			}
		default:
			if s, ok := sanitizeMetadataString(value); ok {
				out[key] = s
			}
		}
	}
	return out
}

func sanitizeStringSlice(value any) []string {
	items, ok := value.([]string)
	if !ok {
		raw, ok := value.([]any)
		if !ok {
			return nil
		}
		items = make([]string, 0, len(raw))
		for _, item := range raw {
			s, ok := item.(string)
			if !ok {
				continue
			}
			items = append(items, s)
		}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := sanitizeMetadataString(item)
		if !ok {
			continue
		}
		out = append(out, s)
		if len(out) >= maxAuditRoleIDs {
			break
		}
	}
	return out
}

func sanitizeMetadataString(value any) (string, bool) {
	s, ok := value.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "eyj") {
		return "", false
	}
	for _, needle := range forbiddenAuditSubstrings {
		if strings.Contains(lower, needle) {
			return "", false
		}
	}
	if utf8.RuneCountInString(s) > maxAuditMetadataLen {
		s = string([]rune(s)[:maxAuditMetadataLen])
	}
	return s, true
}
