package httpapi

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	// DefaultPage is the default 1-based page.
	DefaultPage = 1
	// DefaultPageSize is the default page size.
	DefaultPageSize = 10
	// MaxPageSize is the hard upper bound for pageSize (documented in OpenAPI).
	MaxPageSize = 100
)

// Pagination holds validated list parameters.
type Pagination struct {
	Page     int
	PageSize int
	Offset   int
}

// ParsePagination reads page/pageSize query params with defaults and hard limits.
// Invalid integers, non-positive values, pageSize above MaxPageSize, or overflow return ErrInvalidQuery.
func ParsePagination(c *gin.Context) (Pagination, error) {
	pageRaw := c.DefaultQuery("page", strconv.Itoa(DefaultPage))
	sizeRaw := c.DefaultQuery("pageSize", strconv.Itoa(DefaultPageSize))

	page, err := strconv.Atoi(pageRaw)
	if err != nil {
		return Pagination{}, fmt.Errorf("%w: page must be a positive integer", ErrInvalidQuery)
	}
	size, err := strconv.Atoi(sizeRaw)
	if err != nil {
		return Pagination{}, fmt.Errorf("%w: pageSize must be a positive integer", ErrInvalidQuery)
	}
	if page < 1 {
		return Pagination{}, fmt.Errorf("%w: page must be >= 1", ErrInvalidQuery)
	}
	if size < 1 {
		return Pagination{}, fmt.Errorf("%w: pageSize must be >= 1", ErrInvalidQuery)
	}
	if size > MaxPageSize {
		return Pagination{}, fmt.Errorf("%w: pageSize must be <= %d", ErrInvalidQuery, MaxPageSize)
	}

	// Overflow guard for offset.
	if page > 1 && size > (1<<31-1)/(page-1) {
		return Pagination{}, fmt.Errorf("%w: page/pageSize combination overflows", ErrInvalidQuery)
	}
	offset := (page - 1) * size
	return Pagination{Page: page, PageSize: size, Offset: offset}, nil
}

// ParsePaginationOrWrite parses pagination or writes a problem response.
func ParsePaginationOrWrite(c *gin.Context) (Pagination, bool) {
	p, err := ParsePagination(c)
	if err != nil {
		WriteProblem(c, MapError(err, RequestID(c)))
		return Pagination{}, false
	}
	return p, true
}

// TotalPages computes ceil(total/pageSize) safely.
func TotalPages(total, pageSize int) int {
	if pageSize <= 0 {
		return 0
	}
	return (total + pageSize - 1) / pageSize
}
