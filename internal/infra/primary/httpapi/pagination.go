package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gin-gonic/gin"
)

const (
	// DefaultPageSize is the default keyset page size.
	DefaultPageSize = 10
	// MaxPageSize is the hard upper bound for limit (documented in OpenAPI).
	MaxPageSize   = 100
	cursorVersion = 1
	maxCursorSize = 1024
)

// CursorPagination holds validated keyset list parameters.
type CursorPagination struct {
	Limit int
	After *authBaseDomain.CursorKey
}

type cursorPayload struct {
	Version   int    `json:"v"`
	CreatedAt string `json:"createdAt"`
	ID        string `json:"id"`
}

// ParseCursorPagination reads limit/cursor query params with defaults and hard limits.
func ParseCursorPagination(c *gin.Context) (CursorPagination, error) {
	limitRaw := c.DefaultQuery("limit", strconv.Itoa(DefaultPageSize))
	limit, err := strconv.Atoi(limitRaw)
	if err != nil {
		return CursorPagination{}, fmt.Errorf("%w: limit must be a positive integer", ErrInvalidQuery)
	}
	if limit < 1 {
		return CursorPagination{}, fmt.Errorf("%w: limit must be >= 1", ErrInvalidQuery)
	}
	if limit > MaxPageSize {
		return CursorPagination{}, fmt.Errorf("%w: limit must be <= %d", ErrInvalidQuery, MaxPageSize)
	}

	rawCursor := c.Query("cursor")
	if rawCursor == "" {
		return CursorPagination{Limit: limit}, nil
	}
	after, err := DecodeCursor(rawCursor)
	if err != nil {
		return CursorPagination{}, err
	}
	return CursorPagination{Limit: limit, After: &after}, nil
}

// ParseCursorPaginationOrWrite parses pagination or writes a problem response.
func ParseCursorPaginationOrWrite(c *gin.Context) (CursorPagination, bool) {
	p, err := ParseCursorPagination(c)
	if err != nil {
		WriteProblem(c, MapError(err, RequestID(c)))
		return CursorPagination{}, false
	}
	return p, true
}

// EncodeCursor serializes a keyset cursor as opaque Base64URL.
func EncodeCursor(key authBaseDomain.CursorKey) (string, error) {
	if key.CreatedAt.IsZero() || !isUUID(key.ID) {
		return "", fmt.Errorf("%w: cursor key is invalid", ErrInvalidQuery)
	}
	payload, err := json.Marshal(cursorPayload{
		Version:   cursorVersion,
		CreatedAt: key.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:        key.ID,
	})
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// DecodeCursor validates and decodes an opaque Base64URL cursor.
func DecodeCursor(raw string) (authBaseDomain.CursorKey, error) {
	if len(raw) > maxCursorSize {
		return authBaseDomain.CursorKey{}, fmt.Errorf("%w: cursor is too large", ErrInvalidQuery)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return authBaseDomain.CursorKey{}, fmt.Errorf("%w: cursor encoding is invalid", ErrInvalidQuery)
	}

	var payload cursorPayload
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return authBaseDomain.CursorKey{}, fmt.Errorf("%w: cursor payload is invalid", ErrInvalidQuery)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return authBaseDomain.CursorKey{}, fmt.Errorf("%w: cursor payload is invalid", ErrInvalidQuery)
	}
	if payload.Version != cursorVersion {
		return authBaseDomain.CursorKey{}, fmt.Errorf("%w: cursor version is unsupported", ErrInvalidQuery)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, payload.CreatedAt)
	if err != nil || createdAt.IsZero() || !isUUID(payload.ID) {
		return authBaseDomain.CursorKey{}, fmt.Errorf("%w: cursor values are invalid", ErrInvalidQuery)
	}
	return authBaseDomain.CursorKey{CreatedAt: createdAt.UTC(), ID: payload.ID}, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("unexpected trailing JSON")
	}
	return err
}

func isUUID(value string) bool {
	compact := strings.ReplaceAll(value, "-", "")
	if len(value) != 36 || len(compact) != 32 {
		return false
	}
	if value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(compact)
	return err == nil
}
