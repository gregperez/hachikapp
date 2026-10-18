package sessions

import "time"

// SessionResponse is the public view of a refresh family.
// @Description Active session. Never includes hashes or JWTs.
type SessionResponse struct {
	FamilyID    string    `json:"familyId"`
	Current     bool      `json:"current"`
	IssuedAt    time.Time `json:"issuedAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	ClientLabel string    `json:"clientLabel"`
	IPPrefix    string    `json:"ipPrefix,omitempty"`
}

// SessionsPageResponse is a keyset page of sessions.
// @Description Paginated list of the caller's active sessions
type SessionsPageResponse struct {
	Limit      int               `json:"limit"`
	HasNext    bool              `json:"hasNext"`
	NextCursor string            `json:"nextCursor,omitempty"`
	Data       []SessionResponse `json:"data"`
}

// SessionRevokeResponse reports whether the caller's current family was revoked.
// @Description Outcome of a self-service session revocation
type SessionRevokeResponse struct {
	RevokedCurrent bool `json:"revokedCurrent"`
}
