package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// DefaultMaxBodyBytes is the default request body size limit (1 MiB).
const DefaultMaxBodyBytes int64 = 1 << 20

var validate = newBindingValidator()

func newBindingValidator() *validator.Validate {
	v := validator.New()
	// Controllers use gin-style `binding` tags; keep that contract.
	v.SetTagName("binding")
	return v
}

// BindJSON decodes JSON into dst without panicking.
// It rejects unknown fields, empty bodies, oversized bodies, and runs struct validation.
func BindJSON(c *gin.Context, dst any) error {
	return BindJSONLimited(c, dst, DefaultMaxBodyBytes)
}

// BindJSONLimited is BindJSON with an explicit size limit.
func BindJSONLimited(c *gin.Context, dst any, maxBytes int64) error {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return ErrEmptyBody
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}

	body := http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	defer body.Close()

	raw, err := io.ReadAll(body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) || isBodyTooLarge(err) {
			return ErrBodyTooLarge
		}
		return ErrInvalidJSON
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return ErrEmptyBody
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return classifyJSONError(err)
	}
	// Reject trailing junk after the first value.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return ErrInvalidJSON
	}

	if err := validate.Struct(dst); err != nil {
		return classifyValidationError(err)
	}
	return nil
}

func classifyJSONError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, io.EOF) {
		return ErrEmptyBody
	}
	var syn *json.SyntaxError
	if errors.As(err, &syn) {
		return ErrInvalidJSON
	}
	var typ *json.UnmarshalTypeError
	if errors.As(err, &typ) {
		return ErrInvalidJSON
	}
	msg := err.Error()
	if strings.Contains(msg, "unknown field") {
		return ErrUnknownField
	}
	return ErrInvalidJSON
}

func classifyValidationError(err error) error {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return &ValidationError{Fields: []FieldError{{Field: "_", Code: "invalid"}}}
	}
	fields := make([]FieldError, 0, len(ve))
	for _, fe := range ve {
		code := fe.Tag()
		if code == "" {
			code = "invalid"
		}
		fields = append(fields, FieldError{Field: fe.Field(), Code: code})
	}
	return &ValidationError{Fields: fields}
}

func isBodyTooLarge(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "http: request body too large") ||
		strings.Contains(msg, "request body too large")
}

// BindAndWrite binds JSON or writes the corresponding problem response.
// Returns false when binding failed (response already written).
func BindAndWrite(c *gin.Context, dst any) bool {
	if err := BindJSON(c, dst); err != nil {
		WriteProblem(c, MapError(err, RequestID(c)))
		return false
	}
	return true
}
