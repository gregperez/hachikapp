package httpapi

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/WilsonSayago/base-golang/internal/infra/logging"
	"github.com/gin-gonic/gin"
)

// Recovery returns middleware that catches panics, logs the stack internally,
// and responds with a redacted 500 problem+json. User-facing bind/validation
// errors must never reach this path.
// A nil logger uses slog.Default().
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	lg := logging.OrDefault(logger)
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				rid := RequestID(c)
				lg.Error("http panic recovered",
					"request_id", rid,
					"panic", rec,
					"stack", string(debug.Stack()),
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
				)
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
