package ginadapter

import (
	"context"

	authBaseCore "github.com/WilsonSayago/authBase/v3/core"
	"github.com/gin-gonic/gin"
)

// Context adapts *gin.Context to authBase/v3 core.Context.
// Gin v1.12 widened Keys to any; we re-expose string-keyed Get/Set for authBase.
type Context struct {
	*gin.Context
}

// Compile-time check that the adapter satisfies authBase's Context contract.
var _ authBaseCore.Context = (*Context)(nil)

// RequestContext returns the request-scoped context.
func (c *Context) RequestContext() context.Context {
	if c == nil || c.Context == nil || c.Context.Request == nil {
		return context.Background()
	}
	return c.Context.Request.Context()
}

// Get returns a string-keyed value for authBase (Gin stores keys as any).
func (c *Context) Get(key string) (any, bool) {
	if c == nil || c.Context == nil {
		return nil, false
	}
	return c.Context.Get(key)
}

// Set stores a string-keyed value for authBase.
func (c *Context) Set(key string, value interface{}) {
	if c == nil || c.Context == nil {
		return
	}
	c.Context.Set(key, value)
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
