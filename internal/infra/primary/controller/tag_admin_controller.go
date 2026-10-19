package controller

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/authz"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto"
	tagin "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/inputs/tags"
	tagout "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/outputs/tags"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/ginadapter"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
)

// TagAdminController manages batches of physical tags. Every route requires
// the TAGS permission (create for POST, read for GETs).
type TagAdminController struct {
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	tags          core.TagUseCase
	baseURL       string
	r             *gin.Engine
}

func NewTagAdminController(
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	tags core.TagUseCase,
	baseURL string,
	r *gin.Engine,
) *TagAdminController {
	return &TagAdminController{authorization: authorization, tags: tags, baseURL: baseURL, r: r}
}

func (ac *TagAdminController) RunController() {
	if ac.tags == nil {
		return
	}
	group := ac.r.Group("/v1/admin/tag-batches")
	group.Use(ginadapter.Middleware(ac.authorization.AuthorizeJWT()))
	{
		group.GET("", ginadapter.Middleware(ac.authorization.PoliciesGuard(
			ginadapter.Handler(ac.List()), nil, authz.EntityTags, authz.Read)))
		group.POST("", ginadapter.Middleware(ac.authorization.PoliciesGuard(
			ginadapter.Handler(ac.Create()), nil, authz.EntityTags, authz.Create)))
		group.GET("/:id/codes.csv", ginadapter.Middleware(ac.authorization.PoliciesGuard(
			ginadapter.Handler(ac.CodesCSV()), nil, authz.EntityTags, authz.Read)))
	}
}

// List godoc
// @Summary List tag batches
// @Description Newest first, keyset paginated. Requires TAGS.read.
// @Tags Tag batches
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Page size (1-100)" default(10) maximum(100)
// @Param cursor query string false "Opaque cursor returned by the previous page"
// @Success 200 {object} dto.ResponseDto{data=tagout.TagBatchesPageResponse}
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Router /v1/admin/tag-batches [get]
func (ac *TagAdminController) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		request, ok := handlePagination(c)
		if !ok {
			return
		}
		page, err := ac.tags.ListBatches(c.Request.Context(), request)
		if err != nil {
			writeTagError(c, err)
			return
		}
		out := make([]tagout.TagBatchResponse, 0, len(page.Items))
		for _, batch := range page.Items {
			out = append(out, toTagBatchResponse(batch))
		}
		nextCursor := ""
		if page.HasNext && len(page.Items) > 0 {
			last := page.Items[len(page.Items)-1]
			nextCursor, err = httpapi.EncodeCursor(authBaseDomain.CursorKey{CreatedAt: last.CreatedAt, ID: last.ID})
			if err != nil {
				httpapi.WriteProblem(c, httpapi.MapError(fmt.Errorf("encode tag batches cursor: %v", err), httpapi.RequestID(c)))
				return
			}
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: tagout.TagBatchesPageResponse{
			Limit:      request.Limit,
			HasNext:    page.HasNext,
			NextCursor: nextCursor,
			Data:       out,
		}})
	}
}

// Create godoc
// @Summary Generate a batch of physical tags
// @Description Creates quantity (1-1000) unassigned tags with random codes. Requires TAGS.create.
// @Tags Tag batches
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body tagin.CreateTagBatchRequest true "Batch"
// @Success 201 {object} dto.ResponseDto{data=tagout.TagBatchResponse}
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Router /v1/admin/tag-batches [post]
func (ac *TagAdminController) Create() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(ac.authorization, c)
		if !ok {
			return
		}
		var req tagin.CreateTagBatchRequest
		if !httpapi.BindAndWrite(c, &req) {
			return
		}
		batch, err := ac.tags.CreateBatch(c.Request.Context(), actor.GetId(), req.Label, req.Quantity)
		if err != nil {
			writeTagError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto.ResponseDto{Status: http.StatusCreated, Message: "Created", Data: toTagBatchResponse(batch)})
	}
}

// CodesCSV godoc
// @Summary Download the codes of a tag batch
// @Description CSV with header `code,url`, one row per tag; url is PUBLIC_BASE_URL/t/<code>. Requires TAGS.read.
// @Tags Tag batches
// @Security BearerAuth
// @Produce text/csv
// @Param id path string true "Batch ID"
// @Success 200 {file} binary
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Router /v1/admin/tag-batches/{id}/codes.csv [get]
func (ac *TagAdminController) CodesCSV() gin.HandlerFunc {
	return func(c *gin.Context) {
		batchID := c.Param("id")
		tags, err := ac.tags.BatchTags(c.Request.Context(), batchID)
		if err != nil {
			writeTagError(c, err)
			return
		}
		// A batch always has at least one tag: no rows means no such batch.
		if len(tags) == 0 {
			httpapi.WriteProblem(c, httpapi.NotFound("Tag batch not found", httpapi.RequestID(c)))
			return
		}
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		_ = w.Write([]string{"code", "url"})
		for _, tag := range tags {
			_ = w.Write([]string{tag.Code, tagURL(ac.baseURL, tag.Code)})
		}
		w.Flush()
		if err := w.Error(); err != nil {
			httpapi.WriteProblem(c, httpapi.Internal(httpapi.RequestID(c)))
			return
		}
		c.Header("Content-Disposition", `attachment; filename="placas-`+batchID+`.csv"`)
		c.Header("Cache-Control", "private, no-store")
		c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
	}
}

func toTagBatchResponse(batch domain.TagBatch) tagout.TagBatchResponse {
	return tagout.TagBatchResponse{
		ID:        batch.ID,
		Label:     batch.Label,
		Quantity:  batch.Quantity,
		CreatedAt: formatPetTime(batch.CreatedAt),
	}
}
