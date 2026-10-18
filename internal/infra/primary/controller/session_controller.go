package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	authBaseServices "github.com/WilsonSayago/authBase/v4/core/services"
	"github.com/WilsonSayago/base-golang/internal/core"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto"
	sessionout "github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto/outputs/sessions"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/ginadapter"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/httpapi"
	"github.com/gin-gonic/gin"
)

const refreshTokenHeader = "X-Refresh-Token"

type SessionController struct {
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	sessions      core.SessionUseCase
	tokens        *authBaseServices.TokenManager
	r             *gin.Engine
	t             config.ITranslate
}

func NewSessionController(
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	sessions core.SessionUseCase,
	tokens *authBaseServices.TokenManager,
	r *gin.Engine,
	t config.ITranslate,
) *SessionController {
	return &SessionController{
		authorization: authorization,
		sessions:      sessions,
		tokens:        tokens,
		r:             r,
		t:             t,
	}
}

func (sc *SessionController) RunController() {
	if sc.sessions == nil {
		return
	}
	protected := sc.r.Group("/v1/users/me/sessions")
	protected.Use(ginadapter.Middleware(sc.authorization.AuthorizeJWT()))
	{
		protected.GET("", sc.List())
		protected.DELETE("", sc.RevokeAll())
		protected.DELETE("/:familyId", sc.RevokeOne())
	}
}

// List godoc
// @Summary List own active sessions
// @Description Returns the caller's active refresh families. The current family is derived from the validated refresh token, never from client-supplied IDs.
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Page size (1-100)" default(10) maximum(100)
// @Param cursor query string false "Opaque cursor returned by the previous page"
// @Param X-Refresh-Token header string false "Current refresh JWT used only to mark this session"
// @Success 200 {object} dto.ResponseDto{data=sessionout.SessionsPageResponse}
// @Failure 401 {object} httpapi.Problem
// @Failure 400 {object} httpapi.Problem
// @Router /v1/users/me/sessions [get]
func (sc *SessionController) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(sc.authorization, c)
		if !ok {
			return
		}
		request, ok := parseSessionPage(c)
		if !ok {
			return
		}
		page, err := sc.sessions.ListMine(c.Request.Context(), actor.GetId(), sc.currentFamily(c, actor.GetId()), request)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		items := make([]sessionout.SessionResponse, 0, len(page.Items))
		for _, item := range page.Items {
			items = append(items, toSessionResponse(item))
		}
		nextCursor := ""
		if page.HasNext && len(page.Items) > 0 {
			last := page.Items[len(page.Items)-1]
			encoded, err := httpapi.EncodeSessionCursor(last.LastSeenAt, last.FamilyID)
			if err != nil {
				httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
				return
			}
			nextCursor = encoded
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data: sessionout.SessionsPageResponse{
				Limit:      request.Limit,
				HasNext:    page.HasNext,
				NextCursor: nextCursor,
				Data:       items,
			},
		})
	}
}

// RevokeAll godoc
// @Summary Revoke every own session
// @Description Revokes all refresh families for the caller, including the current one. The BFF must clear cookies.
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.ResponseDto{data=sessionout.SessionRevokeResponse}
// @Failure 401 {object} httpapi.Problem
// @Router /v1/users/me/sessions [delete]
func (sc *SessionController) RevokeAll() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(sc.authorization, c)
		if !ok {
			return
		}
		if err := sc.sessions.RevokeAllMine(c.Request.Context(), actor.GetId()); err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    sessionout.SessionRevokeResponse{RevokedCurrent: true},
		})
	}
}

// RevokeOne godoc
// @Summary Revoke one own session
// @Description Revokes a single refresh family owned by the caller. Foreign family IDs are treated as a successful no-op.
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Param familyId path string true "Refresh family identifier"
// @Param X-Refresh-Token header string false "Current refresh JWT used only to detect signing out this device"
// @Success 200 {object} dto.ResponseDto{data=sessionout.SessionRevokeResponse}
// @Failure 401 {object} httpapi.Problem
// @Router /v1/users/me/sessions/{familyId} [delete]
func (sc *SessionController) RevokeOne() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(sc.authorization, c)
		if !ok {
			return
		}
		familyID := strings.TrimSpace(c.Param("familyId"))
		revokedCurrent, err := sc.sessions.RevokeMine(c.Request.Context(), actor.GetId(), familyID, sc.currentFamily(c, actor.GetId()))
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    sessionout.SessionRevokeResponse{RevokedCurrent: revokedCurrent},
		})
	}
}

func (sc *SessionController) currentFamily(c *gin.Context, actorID string) string {
	if sc.tokens == nil {
		return ""
	}
	refresh := strings.TrimSpace(c.GetHeader(refreshTokenHeader))
	if refresh == "" {
		return ""
	}
	claims, err := sc.tokens.ParseRefresh(refresh)
	if err != nil || claims.Subject != actorID || claims.FamilyID == "" {
		return ""
	}
	return claims.FamilyID
}

func parseSessionPage(c *gin.Context) (authBaseDomain.PageRequest, bool) {
	limitRaw := c.DefaultQuery("limit", strconv.Itoa(httpapi.DefaultPageSize))
	limit, err := strconv.Atoi(limitRaw)
	if err != nil || limit < 1 || limit > httpapi.MaxPageSize {
		httpapi.WriteProblem(c, httpapi.MapError(
			fmt.Errorf("%w: limit must be between 1 and %d", httpapi.ErrInvalidQuery, httpapi.MaxPageSize),
			httpapi.RequestID(c),
		))
		return authBaseDomain.PageRequest{}, false
	}
	request := authBaseDomain.PageRequest{Limit: limit}
	rawCursor := c.Query("cursor")
	if rawCursor == "" {
		return request, true
	}
	lastSeen, familyID, err := httpapi.DecodeSessionCursor(rawCursor)
	if err != nil {
		httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
		return authBaseDomain.PageRequest{}, false
	}
	request.After = &authBaseDomain.CursorKey{CreatedAt: lastSeen, ID: familyID}
	return request, true
}

func toSessionResponse(item domain.Session) sessionout.SessionResponse {
	return sessionout.SessionResponse{
		FamilyID:    item.FamilyID,
		Current:     item.Current,
		IssuedAt:    item.IssuedAt.UTC(),
		LastSeenAt:  item.LastSeenAt.UTC(),
		ExpiresAt:   item.ExpiresAt.UTC(),
		ClientLabel: item.ClientLabel,
		IPPrefix:    item.IPPrefix,
	}
}
