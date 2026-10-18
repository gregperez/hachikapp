package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type sessionCursorPayload struct {
	Version    int    `json:"v"`
	LastSeenAt string `json:"lastSeenAt"`
	FamilyID   string `json:"familyId"`
}

const maxFamilyIDLen = 128

// EncodeSessionCursor serializes last-seen + family id as an opaque keyset cursor.
func EncodeSessionCursor(lastSeenAt time.Time, familyID string) (string, error) {
	if lastSeenAt.IsZero() || familyID == "" || len(familyID) > maxFamilyIDLen {
		return "", fmt.Errorf("%w: session cursor key is invalid", ErrInvalidQuery)
	}
	payload, err := json.Marshal(sessionCursorPayload{
		Version:    cursorVersion,
		LastSeenAt: lastSeenAt.UTC().Format(time.RFC3339Nano),
		FamilyID:   familyID,
	})
	if err != nil {
		return "", fmt.Errorf("encode session cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// DecodeSessionCursor validates a session list cursor.
func DecodeSessionCursor(raw string) (time.Time, string, error) {
	if raw == "" {
		return time.Time{}, "", nil
	}
	if len(raw) > maxCursorSize {
		return time.Time{}, "", fmt.Errorf("%w: cursor is too large", ErrInvalidQuery)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("%w: cursor encoding is invalid", ErrInvalidQuery)
	}
	var payload sessionCursorPayload
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return time.Time{}, "", fmt.Errorf("%w: cursor payload is invalid", ErrInvalidQuery)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return time.Time{}, "", fmt.Errorf("%w: cursor payload is invalid", ErrInvalidQuery)
	}
	if payload.Version != cursorVersion || payload.FamilyID == "" || len(payload.FamilyID) > maxFamilyIDLen {
		return time.Time{}, "", fmt.Errorf("%w: cursor values are invalid", ErrInvalidQuery)
	}
	if strings.ContainsAny(payload.FamilyID, " \t\n") {
		return time.Time{}, "", fmt.Errorf("%w: cursor values are invalid", ErrInvalidQuery)
	}
	lastSeen, err := time.Parse(time.RFC3339Nano, payload.LastSeenAt)
	if err != nil || lastSeen.IsZero() {
		return time.Time{}, "", fmt.Errorf("%w: cursor values are invalid", ErrInvalidQuery)
	}
	return lastSeen.UTC(), payload.FamilyID, nil
}
