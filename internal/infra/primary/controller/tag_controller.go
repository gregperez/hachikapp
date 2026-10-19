package controller

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto"
	tagin "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/inputs/tags"
	tagout "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/outputs/tags"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/ginadapter"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
)

// TagController exposes the tags and scan history of the authenticated
// account's pets. Ownership is enforced by the use cases: other accounts'
// pets and tags are 404.
type TagController struct {
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	tags          core.TagUseCase
	scans         core.PublicTagUseCase
	baseURL       string
	r             *gin.Engine
}

// NewTagController builds the owner routes. baseURL is PUBLIC_BASE_URL, used
// to build each tag's public link (baseURL + "/t/" + code).
func NewTagController(
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	tags core.TagUseCase,
	scans core.PublicTagUseCase,
	baseURL string,
	r *gin.Engine,
) *TagController {
	return &TagController{authorization: authorization, tags: tags, scans: scans, baseURL: baseURL, r: r}
}

func (tc *TagController) RunController() {
	if tc.tags == nil || tc.scans == nil {
		return
	}
	jwt := ginadapter.Middleware(tc.authorization.AuthorizeJWT())
	pets := tc.r.Group("/v1/pets", jwt)
	{
		pets.GET("/:id/tags", tc.List())
		pets.POST("/:id/tags", tc.CreateDigital())
		pets.POST("/:id/tags/:tagId/disable", tc.Disable())
		pets.GET("/:id/scans", tc.Scans())
	}
	tc.r.POST("/v1/tags/:code/activate", jwt, tc.Activate())
}

// List godoc
// @Summary List the tags of one of my pets
// @Tags Tags
// @Security BearerAuth
// @Produce json
// @Param id path string true "Pet ID"
// @Success 200 {object} dto.ResponseDto{data=[]tagout.TagResponse}
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Router /v1/pets/{id}/tags [get]
func (tc *TagController) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(tc.authorization, c)
		if !ok {
			return
		}
		tags, err := tc.tags.ListPetTags(c.Request.Context(), actor.GetId(), c.Param("id"))
		if err != nil {
			writeTagError(c, err)
			return
		}
		out := make([]tagout.TagResponse, 0, len(tags))
		for _, tag := range tags {
			out = append(out, toTagResponse(tc.baseURL, tag))
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: out})
	}
}

// CreateDigital godoc
// @Summary Create a digital tag for one of my pets
// @Description Creates an already active digital tag (QR) linked to the pet. A pet can have up to 5 active tags.
// @Tags Tags
// @Security BearerAuth
// @Produce json
// @Param id path string true "Pet ID"
// @Success 201 {object} dto.ResponseDto{data=tagout.TagResponse}
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Failure 409 {object} httpapi.Problem
// @Router /v1/pets/{id}/tags [post]
func (tc *TagController) CreateDigital() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(tc.authorization, c)
		if !ok {
			return
		}
		tag, err := tc.tags.CreateDigital(c.Request.Context(), actor.GetId(), c.Param("id"))
		if err != nil {
			writeTagError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto.ResponseDto{Status: http.StatusCreated, Message: "Created", Data: toTagResponse(tc.baseURL, tag)})
	}
}

// Disable godoc
// @Summary Disable a tag of one of my pets
// @Description The public page of a disabled tag shows no data. There is no reactivation.
// @Tags Tags
// @Security BearerAuth
// @Produce json
// @Param id path string true "Pet ID"
// @Param tagId path string true "Tag ID"
// @Success 200 {object} dto.ResponseDto{data=tagout.TagResponse}
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Router /v1/pets/{id}/tags/{tagId}/disable [post]
func (tc *TagController) Disable() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(tc.authorization, c)
		if !ok {
			return
		}
		tag, err := tc.tags.Disable(c.Request.Context(), actor.GetId(), c.Param("id"), c.Param("tagId"))
		if err != nil {
			writeTagError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: toTagResponse(tc.baseURL, tag)})
	}
}

// Activate godoc
// @Summary Activate a physical tag for one of my pets
// @Description Links an unassigned tag to the pet. 409 when the tag is already linked or disabled, or when the pet already has 5 active tags; 404 when the pet is not mine or the code does not exist.
// @Tags Tags
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param code path string true "Tag code (case-insensitive; spaces and hyphens ignored)"
// @Param body body tagin.ActivateTagRequest true "Pet to link"
// @Success 200 {object} dto.ResponseDto{data=tagout.TagResponse}
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Failure 409 {object} httpapi.Problem
// @Router /v1/tags/{code}/activate [post]
func (tc *TagController) Activate() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(tc.authorization, c)
		if !ok {
			return
		}
		var req tagin.ActivateTagRequest
		if !httpapi.BindAndWrite(c, &req) {
			return
		}
		tag, err := tc.tags.Activate(c.Request.Context(), actor.GetId(), c.Param("code"), req.PetID)
		if err != nil {
			writeTagError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: toTagResponse(tc.baseURL, tag)})
	}
}

// Scans godoc
// @Summary List the scans of one of my pets' tags
// @Description Newest first, each with the messages finders left (oldest first). Visitor and token hashes and IP prefixes are never returned.
// @Tags Tags
// @Security BearerAuth
// @Produce json
// @Param id path string true "Pet ID"
// @Param limit query int false "Page size (1-100)" default(10) maximum(100)
// @Param cursor query string false "Opaque cursor returned by the previous page"
// @Success 200 {object} dto.ResponseDto{data=tagout.ScansPageResponse}
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Router /v1/pets/{id}/scans [get]
func (tc *TagController) Scans() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(tc.authorization, c)
		if !ok {
			return
		}
		request, ok := handlePagination(c)
		if !ok {
			return
		}
		page, err := tc.scans.ListPetScans(c.Request.Context(), actor.GetId(), c.Param("id"), request)
		if err != nil {
			writeTagError(c, err)
			return
		}
		out := make([]tagout.ScanResponse, 0, len(page.Items))
		for _, scan := range page.Items {
			out = append(out, toScanResponse(scan))
		}
		nextCursor := ""
		if page.HasNext && len(page.Items) > 0 {
			last := page.Items[len(page.Items)-1]
			nextCursor, err = httpapi.EncodeCursor(authBaseDomain.CursorKey{CreatedAt: last.ScannedAt, ID: last.ID})
			if err != nil {
				httpapi.WriteProblem(c, httpapi.MapError(fmt.Errorf("encode scans cursor: %v", err), httpapi.RequestID(c)))
				return
			}
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: tagout.ScansPageResponse{
			Limit:      request.Limit,
			HasNext:    page.HasNext,
			NextCursor: nextCursor,
			Data:       out,
		}})
	}
}

func writeTagError(c *gin.Context, err error) {
	requestID := httpapi.RequestID(c)
	var invalid *domain.PetValidationError
	switch {
	case errors.As(err, &invalid):
		httpapi.WriteProblem(c, httpapi.Validation("Request contains invalid fields", requestID, fieldErrors(invalid)))
	case errors.Is(err, domain.ErrTagNotAssignable):
		httpapi.WriteProblem(c, httpapi.Conflict("This tag is already linked", requestID))
	case errors.Is(err, domain.ErrTagLimitReached):
		httpapi.WriteProblem(c, httpapi.Conflict("The pet already has 5 active tags", requestID))
	default:
		httpapi.WriteProblem(c, httpapi.MapError(err, requestID))
	}
}

// tagURL is the link printed on the NFC tag and QR.
func tagURL(baseURL, code string) string {
	return baseURL + "/t/" + code
}

// toTagResponse omits OwnerID and BatchID on purpose.
func toTagResponse(baseURL string, tag domain.PetTag) tagout.TagResponse {
	return tagout.TagResponse{
		ID:          tag.ID,
		Code:        tag.Code,
		Kind:        string(tag.Kind),
		Status:      string(tag.Status),
		PetID:       tag.PetID,
		ActivatedAt: formatOptionalTime(tag.ActivatedAt),
		DisabledAt:  formatOptionalTime(tag.DisabledAt),
		CreatedAt:   formatPetTime(tag.CreatedAt),
		URL:         tagURL(baseURL, tag.Code),
	}
}

// toScanResponse omits VisitorHash, ScanTokenHash, IPPrefix and internal IDs on purpose.
func toScanResponse(scan domain.ScanWithMessages) tagout.ScanResponse {
	messages := make([]tagout.ScanMessageResponse, 0, len(scan.Messages))
	for _, msg := range scan.Messages {
		messages = append(messages, tagout.ScanMessageResponse{
			ID:          msg.ID,
			Body:        msg.Body,
			FinderName:  msg.FinderName,
			FinderPhone: msg.FinderPhone,
			CreatedAt:   formatPetTime(msg.CreatedAt),
		})
	}
	return tagout.ScanResponse{
		ID:              scan.ID,
		TagCode:         scan.TagCode,
		ScannedAt:       formatPetTime(scan.ScannedAt),
		UserAgentFamily: scan.UserAgentFamily,
		PhoneRevealedAt: formatOptionalTime(scan.PhoneRevealedAt),
		Messages:        messages,
	}
}

func formatOptionalTime(at *time.Time) string {
	if at == nil {
		return ""
	}
	return formatPetTime(*at)
}
