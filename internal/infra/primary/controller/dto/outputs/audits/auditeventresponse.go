package audits

import "time"

// AuditEventResponse is the public view of a persisted audit event.
// @Description Sanitized administrative audit event. Never includes secrets or emails.
type AuditEventResponse struct {
	ID          string         `json:"id"`
	OccurredAt  time.Time      `json:"occurredAt"`
	Category    string         `json:"category"`
	Action      string         `json:"action"`
	Outcome     string         `json:"outcome"`
	ActorID     string         `json:"actorId"`
	SubjectType string         `json:"subjectType"`
	SubjectID   string         `json:"subjectId,omitempty"`
	RequestID   string         `json:"requestId,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// AuditEventsPageResponse is a keyset page of audit events.
// @Description Paginated list of administrative audit events
type AuditEventsPageResponse struct {
	Limit      int                  `json:"limit"`
	HasNext    bool                 `json:"hasNext"`
	NextCursor string               `json:"nextCursor,omitempty"`
	Data       []AuditEventResponse `json:"data"`
}
