package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// LimitRequestBody wraps the request body with MaxBytesReader.
// Oversized bodies fail at read time; BindJSON maps that to 413.
func LimitRequestBody(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	return func(c *gin.Context) {
		if c.Request != nil && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
