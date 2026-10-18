package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/sessionmeta"
)

func attachSessionMeta(c *gin.Context, resolver *ClientOriginResolver) {
	if c == nil || c.Request == nil {
		return
	}
	origin := "unknown"
	if resolver != nil {
		origin = resolver.Origin(c.Request)
	}
	meta := sessionmeta.FromHTTP(c.GetHeader("User-Agent"), origin)
	c.Request = c.Request.WithContext(sessionmeta.With(c.Request.Context(), meta))
}
