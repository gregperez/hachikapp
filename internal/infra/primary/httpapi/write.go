package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const requestIDKey = "request_id"

// RequestID returns the request id set by middleware, or empty.
func RequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if id, ok := c.Get(requestIDKey); ok {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return c.GetHeader("X-Request-ID")
}

// WriteProblem writes a problem+json response and aborts the chain.
func WriteProblem(c *gin.Context, p Problem) {
	if p.RequestID == "" {
		p.RequestID = RequestID(c)
	}
	c.Header("Content-Type", ContentTypeProblemJSON)
	c.AbortWithStatusJSON(p.Status, p)
}

// WriteOK writes a successful JSON envelope (legacy ResponseDto shape preserved for success).
func WriteOK(c *gin.Context, status int, message string, data any) {
	if message == "" {
		message = "Ok"
	}
	c.JSON(status, gin.H{
		"status":  status,
		"message": message,
		"data":    data,
	})
}

// WriteNoContent responds with 204.
func WriteNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
