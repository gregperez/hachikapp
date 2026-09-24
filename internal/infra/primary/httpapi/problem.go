package httpapi

import "net/http"

// Problem is an RFC 7807–style error payload (application/problem+json).
// It never includes stack traces, SQL, DSN, hashes, or tokens.
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Code      string       `json:"code"`
	Detail    string       `json:"detail,omitempty"`
	RequestID string       `json:"requestId,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

// FieldError describes a single invalid request field.
type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// Stable problem codes.
const (
	CodeBadRequest      = "BAD_REQUEST"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeForbidden       = "FORBIDDEN"
	CodeNotFound        = "NOT_FOUND"
	CodeConflict        = "CONFLICT"
	CodeValidation      = "VALIDATION_ERROR"
	CodeInternal        = "INTERNAL_ERROR"
	CodePayloadTooLarge = "PAYLOAD_TOO_LARGE"
)

const (
	typeBase = "https://example.invalid/problems/"

	TypeBadRequest      = typeBase + "bad-request"
	TypeUnauthorized    = typeBase + "unauthorized"
	TypeForbidden       = typeBase + "forbidden"
	TypeNotFound        = typeBase + "not-found"
	TypeConflict        = typeBase + "conflict"
	TypeValidation      = typeBase + "validation"
	TypeInternal        = typeBase + "internal"
	TypePayloadTooLarge = typeBase + "payload-too-large"
)

const ContentTypeProblemJSON = "application/problem+json"

func newProblem(status int, code, typ, title, detail, requestID string, fields []FieldError) Problem {
	return Problem{
		Type:      typ,
		Title:     title,
		Status:    status,
		Code:      code,
		Detail:    detail,
		RequestID: requestID,
		Errors:    fields,
	}
}

func BadRequest(detail, requestID string) Problem {
	return newProblem(http.StatusBadRequest, CodeBadRequest, TypeBadRequest, "Bad request", detail, requestID, nil)
}

func Unauthorized(detail, requestID string) Problem {
	return newProblem(http.StatusUnauthorized, CodeUnauthorized, TypeUnauthorized, "Unauthorized", detail, requestID, nil)
}

func Forbidden(detail, requestID string) Problem {
	return newProblem(http.StatusForbidden, CodeForbidden, TypeForbidden, "Forbidden", detail, requestID, nil)
}

func NotFound(detail, requestID string) Problem {
	return newProblem(http.StatusNotFound, CodeNotFound, TypeNotFound, "Not found", detail, requestID, nil)
}

func Conflict(detail, requestID string) Problem {
	return newProblem(http.StatusConflict, CodeConflict, TypeConflict, "Conflict", detail, requestID, nil)
}

func Validation(detail, requestID string, fields []FieldError) Problem {
	return newProblem(http.StatusUnprocessableEntity, CodeValidation, TypeValidation, "Validation failed", detail, requestID, fields)
}

func Internal(requestID string) Problem {
	return newProblem(http.StatusInternalServerError, CodeInternal, TypeInternal, "Internal server error", "An unexpected error occurred", requestID, nil)
}

func PayloadTooLarge(requestID string) Problem {
	return newProblem(http.StatusRequestEntityTooLarge, CodePayloadTooLarge, TypePayloadTooLarge, "Payload too large", "Request body exceeds the allowed size", requestID, nil)
}
