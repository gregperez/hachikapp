package controller

import (
	"github.com/WilsonSayago/base-golang/internal/infra/config"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/httpapi"
	"github.com/gin-gonic/gin"
)

// handleBind decodes and validates JSON without panicking.
// On failure it writes application/problem+json and returns false.
func handleBind(_ config.ITranslate, c *gin.Context, data interface{}) bool {
	return httpapi.BindAndWrite(c, data)
}

// handlePagination parses page/pageSize or writes a problem response.
// ok=false means the response was already written.
func handlePagination(c *gin.Context) (page, pageSize, offset int, ok bool) {
	p, ok := httpapi.ParsePaginationOrWrite(c)
	if !ok {
		return 0, 0, 0, false
	}
	return p.Page, p.PageSize, p.Offset, true
}
