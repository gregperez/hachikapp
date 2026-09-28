package httpapi

import (
	"errors"

	"github.com/WilsonSayago/base-golang/internal/core/domain"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
)

// MapError converts application/domain errors into a stable Problem.
// Unknown errors become a redacted 500.
func MapError(err error, requestID string) Problem {
	if err == nil {
		return Internal(requestID)
	}

	switch {
	case errors.Is(err, domain.ErrForbidden):
		return Forbidden("You do not have permission to perform this action", requestID)
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, authBaseCore.ErrNotFound):
		return NotFound("The requested resource was not found", requestID)
	case errors.Is(err, domain.ErrLastAdmin):
		return Conflict("Cannot disable or demote the last administrator", requestID)
	case errors.Is(err, domain.ErrWeakPassword):
		return Validation("Password does not meet the current policy", requestID, nil)
	case errors.Is(err, domain.ErrAdminExists), errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrStaleVersion):
		detail := "The resource already exists or conflicts with current state"
		if errors.Is(err, domain.ErrStaleVersion) {
			detail = "The settings were updated by someone else"
		}
		return Conflict(detail, requestID)
	case errors.Is(err, domain.ErrInvalidSettings):
		return Validation(err.Error(), requestID, nil)
	case errors.Is(err, authBaseCore.ErrInvalidCredentials), errors.Is(err, authBaseCore.ErrInvalidRefresh):
		return Unauthorized("Invalid credentials", requestID)
	case errors.Is(err, domain.ErrOIDCDisabled), errors.Is(err, domain.ErrOIDCUnavailable):
		return ServiceUnavailable("Federated login is unavailable", requestID)
	case errors.Is(err, domain.ErrOIDCUnknownProvider):
		return NotFound("The requested identity provider was not found", requestID)
	case errors.Is(err, domain.ErrOIDCRedirectNotAllowed):
		return BadRequest("The requested destination is not allowed", requestID)
	case errors.Is(err, domain.ErrOIDCInvalidState),
		errors.Is(err, domain.ErrOIDCExpired),
		errors.Is(err, domain.ErrOIDCReplay),
		errors.Is(err, domain.ErrOIDCInvalidGrant),
		errors.Is(err, domain.ErrOIDCEmailUnverified):
		return Unauthorized("Federated login could not be completed", requestID)
	case errors.Is(err, domain.ErrOIDCIdentityNotLinked),
		errors.Is(err, domain.ErrOIDCInactiveUser),
		errors.Is(err, domain.ErrOIDCReauthRequired):
		return Unauthorized("Federated login could not be completed", requestID)
	case errors.Is(err, domain.ErrOIDCIdentityConflict):
		return Conflict("This identity is already linked to another account", requestID)
	case errors.Is(err, domain.ErrInvalidAuditFilter), errors.Is(err, ErrInvalidJSON), errors.Is(err, ErrInvalidQuery), errors.Is(err, ErrUnknownField), errors.Is(err, ErrEmptyBody):
		detail := "Invalid request"
		switch {
		case errors.Is(err, ErrEmptyBody):
			detail = "Request body is required"
		case errors.Is(err, ErrUnknownField):
			detail = "Request contains unknown fields"
		case errors.Is(err, domain.ErrInvalidAuditFilter), errors.Is(err, ErrInvalidQuery):
			detail = err.Error()
		default:
			detail = "Request body is not valid JSON"
		}
		return BadRequest(detail, requestID)
	case errors.Is(err, ErrBodyTooLarge):
		return PayloadTooLarge(requestID)
	case errors.Is(err, ErrValidation):
		var ve *ValidationError
		if errors.As(err, &ve) {
			return Validation("Request contains invalid fields", requestID, ve.Fields)
		}
		return Validation("Request contains invalid fields", requestID, nil)
	}

	var ae *AppError
	if errors.As(err, &ae) {
		return newProblem(ae.Status, ae.Code, ae.Type, ae.Title, ae.Detail, requestID, ae.Fields)
	}

	return Internal(requestID)
}

// AppError is an explicit application error with a stable HTTP mapping.
type AppError struct {
	Status int
	Code   string
	Type   string
	Title  string
	Detail string
	Fields []FieldError
	Cause  error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail != "" {
		return e.Detail
	}
	return e.Title
}

func (e *AppError) Unwrap() error { return e.Cause }

// StatusOf returns the HTTP status that MapError would use.
func StatusOf(err error) int {
	return MapError(err, "").Status
}
