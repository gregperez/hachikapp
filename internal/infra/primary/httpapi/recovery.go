package httpapi

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// Recovery returns middleware that catches panics, logs the stack internally,
// and responds with a redacted 500 problem+json. User-facing bind/validation
// errors must never reach this path.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				rid := RequestID(c)
				log.WithFields(log.Fields{
					"request_id": rid,
					"panic":      rec,
					"stack":      string(debug.Stack()),
					"path":       c.Request.URL.Path,
					"method":     c.Request.Method,
				}).Error("http panic recovered")
				if !c.Writer.Written() {
					WriteProblem(c, Internal(rid))
				} else {
					c.AbortWithStatus(http.StatusInternalServerError)
				}
			}
		}()
		c.Next()
	}
}
