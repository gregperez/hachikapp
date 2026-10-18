package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/WilsonSayago/base-golang/internal/core"
	"github.com/WilsonSayago/base-golang/internal/core/authz"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto"
	auditout "github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto/outputs/audits"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/ginadapter"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/httpapi"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gin-gonic/gin"
)

type AuditController struct {
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	audits        core.AuditUseCase
	r             *gin.Engine
	t             config.ITranslate
}

func NewAuditController(
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	audits core.AuditUseCase,
	r *gin.Engine,
	t config.ITranslate,
) *AuditController {
	return &AuditController{authorization: authorization, audits: audits, r: r, t: t}
}

func (ac *AuditController) RunController() {
	if ac.audits == nil {
		return
	}
	protected := ac.r.Group("/v1/audit-events")
	protected.Use(ginadapter.Middleware(ac.authorization.AuthorizeJWT()))
	{
		protected.GET("", ginadapter.Middleware(ac.authorization.PoliciesGuard(
			ginadapter.Handler(ac.List()), nil, authz.EntityAuditEvents, authz.Read)))
	}
}

// List godoc
// @Summary List administrative audit events
// @Description Returns a keyset page of sanitized audit events. Filters are an allowlist; SQL and JSONPath are rejected.
// @Tags Audit
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Page size (1-100)" default(10) maximum(100)
// @Param cursor query string false "Opaque cursor returned by the previous page"
// @Param action query string false "Closed action taxonomy"
// @Param outcome query string false "success or failure"
// @Param actorId query string false "Actor identifier or system"
// @Param subjectId query string false "Subject identifier"
// @Param from query string false "RFC 3339 lower bound"
// @Param to query string false "RFC 3339 upper bound"
// @Success 200 {object} dto.ResponseDto{data=auditout.AuditEventsPageResponse}
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Router /v1/audit-events [get]
func (ac *AuditController) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		pageReq, ok := httpapi.ParseCursorPaginationOrWrite(c)
		if !ok {
			return
		}
		filter, err := parseAuditFilter(c)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		page, err := ac.audits.List(c.Request.Context(), filter, authBaseDomain.PageRequest{
			Limit: pageReq.Limit,
			After: pageReq.After,
		})
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		items := make([]auditout.AuditEventResponse, 0, len(page.Items))
		for _, item := range page.Items {
			items = append(items, toAuditEventResponse(item))
		}
		nextCursor := ""
		if page.HasNext && len(page.Items) > 0 {
			last := page.Items[len(page.Items)-1]
			encoded, err := httpapi.EncodeCursor(authBaseDomain.CursorKey{CreatedAt: last.OccurredAt, ID: last.ID})
			if err != nil {
				httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
				return
			}
			nextCursor = encoded
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data: auditout.AuditEventsPageResponse{
				Limit:      pageReq.Limit,
				HasNext:    page.HasNext,
				NextCursor: nextCursor,
				Data:       items,
			},
		})
	}
}

func parseAuditFilter(c *gin.Context) (domain.AuditListFilter, error) {
	filter := domain.AuditListFilter{
		Action:    strings.TrimSpace(c.Query("action")),
		Outcome:   strings.TrimSpace(c.Query("outcome")),
		ActorID:   strings.TrimSpace(c.Query("actorId")),
		SubjectID: strings.TrimSpace(c.Query("subjectId")),
	}
	fromRaw := strings.TrimSpace(c.Query("from"))
	toRaw := strings.TrimSpace(c.Query("to"))
	if fromRaw != "" {
		parsed, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			return domain.AuditListFilter{}, fmt.Errorf("%w: from must be RFC 3339", httpapi.ErrInvalidQuery)
		}
		filter.From = parsed.UTC()
	}
	if toRaw != "" {
		parsed, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			return domain.AuditListFilter{}, fmt.Errorf("%w: to must be RFC 3339", httpapi.ErrInvalidQuery)
		}
		filter.To = parsed.UTC()
	}
	return filter, nil
}

func toAuditEventResponse(item domain.AuditEvent) auditout.AuditEventResponse {
	return auditout.AuditEventResponse{
		ID:          item.ID,
		OccurredAt:  item.OccurredAt.UTC(),
		Category:    item.Category,
		Action:      item.Action,
		Outcome:     item.Outcome,
		ActorID:     item.ActorID,
		SubjectType: item.SubjectType,
		SubjectID:   item.SubjectID,
		RequestID:   item.RequestID,
		Metadata:    domain.SanitizeAuditMetadata(item.Metadata),
	}
}
