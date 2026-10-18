package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
)

func authenticationLimitKey(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	digest := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(digest[:])
}

func (authC *AuthController) allowOrigin(c *gin.Context) bool {
	if authC.limiter == nil {
		return true
	}
	origin := "unknown"
	if authC.originResolver != nil {
		origin = authC.originResolver.Origin(c.Request)
	} else {
		origin = originString(remoteIP(c.Request.RemoteAddr))
	}
	return authC.allow(c, ports.LoginLimitOrigin, authenticationLimitKey(origin))
}

func (authC *AuthController) allow(
	c *gin.Context,
	scope ports.LoginLimitScope,
	key string,
) bool {
	if authC.limiter == nil {
		return true
	}
	result, err := authC.limiter.Allow(c.Request.Context(), scope, key)
	if err != nil {
		httpapi.WriteProblem(c, httpapi.Internal(httpapi.RequestID(c)))
		return false
	}
	if result.Allowed {
		return true
	}
	seconds := int(math.Ceil(result.RetryAfter.Seconds()))
	if seconds < 1 {
		seconds = int(time.Second.Seconds())
	}
	c.Header("Retry-After", strconv.Itoa(seconds))
	httpapi.WriteProblem(c, httpapi.TooManyRequests(httpapi.RequestID(c)))
	return false
}

func (authC *AuthController) reset(
	c *gin.Context,
	scope ports.LoginLimitScope,
	key string,
) bool {
	if authC.limiter == nil {
		return true
	}
	if err := authC.limiter.Reset(c.Request.Context(), scope, key); err != nil {
		httpapi.WriteProblem(c, httpapi.Internal(httpapi.RequestID(c)))
		return false
	}
	return true
}
