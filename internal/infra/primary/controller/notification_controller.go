package controller

import (
	"fmt"
	"net/http"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto"
	notifout "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/outputs/notifications"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/ginadapter"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
)

// NotificationController exposes the authenticated owner's in-app notifications.
type NotificationController struct {
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	notifications core.NotificationUseCase
	r             *gin.Engine
}

func NewNotificationController(
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	notifications core.NotificationUseCase,
	r *gin.Engine,
) *NotificationController {
	return &NotificationController{authorization: authorization, notifications: notifications, r: r}
}

func (nc *NotificationController) RunController() {
	if nc.notifications == nil {
		return
	}
	jwt := ginadapter.Middleware(nc.authorization.AuthorizeJWT())
	group := nc.r.Group("/v1/notifications", jwt)
	{
		group.GET("", nc.List())
		group.GET("/unread-count", nc.UnreadCount())
		group.POST("/read-all", nc.ReadAll())
		group.POST("/:id/read", nc.MarkRead())
	}
}

// List godoc
// @Summary List my notifications
// @Description Newest first. The payload carries the pet name, tag code, scan time and, while retained (30 days), the shared location.
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Page size (1-100)" default(10) maximum(100)
// @Param cursor query string false "Opaque cursor returned by the previous page"
// @Success 200 {object} dto.ResponseDto{data=notifout.NotificationsPageResponse}
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Router /v1/notifications [get]
func (nc *NotificationController) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(nc.authorization, c)
		if !ok {
			return
		}
		request, ok := handlePagination(c)
		if !ok {
			return
		}
		page, err := nc.notifications.List(c.Request.Context(), actor.GetId(), request)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		out := make([]notifout.NotificationResponse, 0, len(page.Items))
		for _, n := range page.Items {
			out = append(out, toNotificationResponse(n))
		}
		nextCursor := ""
		if page.HasNext && len(page.Items) > 0 {
			last := page.Items[len(page.Items)-1]
			nextCursor, err = httpapi.EncodeCursor(authBaseDomain.CursorKey{CreatedAt: last.CreatedAt, ID: last.ID})
			if err != nil {
				httpapi.WriteProblem(c, httpapi.MapError(fmt.Errorf("encode notifications cursor: %v", err), httpapi.RequestID(c)))
				return
			}
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: notifout.NotificationsPageResponse{
			Limit:      request.Limit,
			HasNext:    page.HasNext,
			NextCursor: nextCursor,
			Data:       out,
		}})
	}
}

// UnreadCount godoc
// @Summary Count my unread notifications
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.ResponseDto{data=notifout.UnreadCountResponse}
// @Failure 401 {object} httpapi.Problem
// @Router /v1/notifications/unread-count [get]
func (nc *NotificationController) UnreadCount() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(nc.authorization, c)
		if !ok {
			return
		}
		count, err := nc.notifications.UnreadCount(c.Request.Context(), actor.GetId())
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: notifout.UnreadCountResponse{Count: count}})
	}
}

// MarkRead godoc
// @Summary Mark one of my notifications as read
// @Description Idempotent. 404 when the notification does not exist or belongs to someone else.
// @Tags Notifications
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 204
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Router /v1/notifications/{id}/read [post]
func (nc *NotificationController) MarkRead() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(nc.authorization, c)
		if !ok {
			return
		}
		if err := nc.notifications.MarkRead(c.Request.Context(), actor.GetId(), c.Param("id")); err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ReadAll godoc
// @Summary Mark all my notifications as read
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.ResponseDto{data=notifout.ReadAllResponse}
// @Failure 401 {object} httpapi.Problem
// @Router /v1/notifications/read-all [post]
func (nc *NotificationController) ReadAll() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(nc.authorization, c)
		if !ok {
			return
		}
		updated, err := nc.notifications.MarkAllRead(c.Request.Context(), actor.GetId())
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: notifout.ReadAllResponse{Updated: updated}})
	}
}

// toNotificationResponse omits UserID and MessageID on purpose.
func toNotificationResponse(n domain.Notification) notifout.NotificationResponse {
	return notifout.NotificationResponse{
		ID:        n.ID,
		Kind:      string(n.Kind),
		PetID:     n.PetID,
		TagID:     n.TagID,
		ScanID:    n.ScanID,
		Payload:   n.Payload,
		ReadAt:    formatOptionalTime(n.ReadAt),
		CreatedAt: formatPetTime(n.CreatedAt),
	}
}
