package ginadapter

import (
	"context"

	"github.com/gin-gonic/gin"
)

// Context adapts *gin.Context to authBase/v3 core.Context.
type Context struct {
	*gin.Context
}

// RequestContext returns the request-scoped context.
func (c *Context) RequestContext() context.Context {
	if c == nil || c.Context == nil || c.Context.Request == nil {
		return context.Background()
	}
	return c.Context.Request.Context()
}

// Wrap returns an authBase-compatible context for an existing Gin context.
func Wrap(c *gin.Context) *Context {
	return &Context{Context: c}
}

// Middleware adapts an authBase middleware/handler that expects *Context into gin.HandlerFunc.
func Middleware(h func(*Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		h(Wrap(c))
	}
}

// Handler adapts a gin.HandlerFunc so it can be passed to PoliciesGuard.
func Handler(h gin.HandlerFunc) func(*Context) {
	return func(c *Context) {
		h(c.Context)
	}
}
