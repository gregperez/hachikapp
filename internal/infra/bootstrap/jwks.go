package bootstrap

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

func registerJWKS(r *gin.Engine, body []byte) {
	if r == nil || len(body) == 0 {
		return
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	r.GET("/.well-known/jwks.json", func(c *gin.Context) {
		if match := c.GetHeader("If-None-Match"); match == etag {
			c.Status(http.StatusNotModified)
			return
		}
		c.Header("Cache-Control", "public, max-age=300")
		c.Header("ETag", etag)
		c.Data(http.StatusOK, "application/json", body)
	})
}
