package controller

import (
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/httpapi"
	"github.com/gin-gonic/gin"
)

// handleBind decodes and validates JSON without panicking.
// On failure it writes application/problem+json and returns false.
func handleBind(_ config.ITranslate, c *gin.Context, data interface{}) bool {
	return httpapi.BindAndWrite(c, data)
}

// handlePagination parses limit/cursor or writes a problem response.
// ok=false means the response was already written.
func handlePagination(c *gin.Context) (authBaseDomain.PageRequest, bool) {
	p, ok := httpapi.ParseCursorPaginationOrWrite(c)
	if !ok {
		return authBaseDomain.PageRequest{}, false
	}
	return authBaseDomain.PageRequest{Limit: p.Limit, After: p.After}, true
}
