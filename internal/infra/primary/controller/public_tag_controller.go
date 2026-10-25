package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto"
	tagin "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/inputs/tags"
	tagout "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/outputs/tags"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
)

const (
	// scanTokenHeader carries the token returned by POST .../scans.
	scanTokenHeader = "X-Scan-Token"
	// publicMaxBodyBytes bounds anonymous JSON bodies (a message is <= 500 chars).
	publicMaxBodyBytes int64 = 8 << 10

	publicNotFoundDetail       = "Tag not found"
	publicLocationSharedDetail = "Location already shared"
	publicRateLimitedDetail    = "Too many requests, try again later"
)

// PublicTagController serves the anonymous tag page ("Ficha pública"). It is
// privacy critical: responses are built only from domain.PublicProfile,
// ScanReceipt and, for /contact, PublicContact, so they never carry the
// owner's address, chip number, surnames or (except /contact) phone.
type PublicTagController struct {
	tags    core.PublicTagUseCase
	limiter ports.LoginLimiter
	origins *ClientOriginResolver
	pepper  []byte
	r       *gin.Engine
}

// NewPublicTagController wires the public routes. pepper keys the rate-limit
// buckets so client IPs are not kept in memory in clear; it must not be empty.
func NewPublicTagController(
	tags core.PublicTagUseCase,
	limiter ports.LoginLimiter,
	origins *ClientOriginResolver,
	pepper []byte,
	r *gin.Engine,
) *PublicTagController {
	if tags != nil && len(pepper) == 0 {
		panic("public tag controller: pepper must not be empty")
	}
	return &PublicTagController{tags: tags, limiter: limiter, origins: origins, pepper: pepper, r: r}
}

func (pc *PublicTagController) RunController() {
	if pc.tags == nil {
		return
	}
	group := pc.r.Group("/v1/public")
	group.Use(publicHeaders)
	{
		group.GET("/tags/:code", pc.limit(ports.PublicTagViewLimit), pc.View())
		group.GET("/tags/:code/photo", pc.limit(ports.PublicTagViewLimit), pc.Photo())
		group.POST("/tags/:code/scans", pc.limit(ports.PublicTagScanLimit), pc.RecordScan())
		group.POST("/scans/:scanId/messages", pc.limit(ports.PublicTagMessageLimit), pc.SendMessage())
		group.PUT("/scans/:scanId/location", pc.limit(ports.PublicTagLocationLimit), pc.ShareLocation())
		group.POST("/scans/:scanId/contact", pc.limit(ports.PublicTagContactLimit), pc.Contact())
	}
}

// publicHeaders runs first so every public response, errors and 429 included,
// is neither cached nor indexed.
func publicHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Robots-Tag", "noindex")
	c.Next()
}

// limit applies the per-IP quota of scope before any other work. The bucket
// key is a keyed hash of the client origin.
func (pc *PublicTagController) limit(scope ports.LoginLimitScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !allowRate(c, pc.limiter, scope, domain.KeyedHash(pc.pepper, pc.clientIP(c)), publicRateLimitedDetail) {
			return
		}
		c.Next()
	}
}

func (pc *PublicTagController) clientIP(c *gin.Context) string {
	if pc.origins != nil {
		return pc.origins.Origin(c.Request)
	}
	return originString(remoteIP(c.Request.RemoteAddr))
}

// View godoc
// @Summary Get the public page of a tag
// @Description No login. Returns the tag state and, when active, only the fields the owner made public (hidden fields are omitted). Never returns phone numbers, address, chip number or surnames. Responses are `Cache-Control: no-store` and `X-Robots-Tag: noindex`. Rate limited per IP (60/min, shared with the photo).
// @Tags Public tags
// @Produce json
// @Param code path string true "Tag code (case-insensitive; spaces and hyphens ignored)"
// @Success 200 {object} dto.ResponseDto{data=tagout.PublicTagViewResponse}
// @Failure 404 {object} httpapi.Problem
// @Failure 429 {object} httpapi.Problem
// @Router /v1/public/tags/{code} [get]
func (pc *PublicTagController) View() gin.HandlerFunc {
	return func(c *gin.Context) {
		code, ok := domain.NormalizeTagCode(c.Param("code"))
		if !ok {
			writePublicTagError(c, domain.ErrNotFound)
			return
		}
		view, err := pc.tags.View(c.Request.Context(), code)
		if err != nil {
			writePublicTagError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: toPublicTagViewResponse(code, view)})
	}
}

// Photo godoc
// @Summary Get the public photo of a tag's pet
// @Description No login. Only when the tag is active and the owner made the photo public; otherwise 404. Rate limited per IP (60/min, shared with the profile).
// @Tags Public tags
// @Produce image/jpeg
// @Produce image/png
// @Param code path string true "Tag code"
// @Param v query string false "Photo version (cache buster)"
// @Success 200 {file} binary
// @Failure 404 {object} httpapi.Problem
// @Failure 429 {object} httpapi.Problem
// @Router /v1/public/tags/{code}/photo [get]
func (pc *PublicTagController) Photo() gin.HandlerFunc {
	return func(c *gin.Context) {
		photo, err := pc.tags.Photo(c.Request.Context(), c.Param("code"))
		if err != nil {
			writePublicTagError(c, err)
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, photo.ContentType, photo.Data)
	}
}

// RecordScan godoc
// @Summary Register a scan of a tag
// @Description No login. Called by the page once loaded (not on server render). visitorId is a UUID kept by the browser; repeated scans by the same visitor within 10 minutes reuse the scan and return a new token. Rate limited per IP (20/hour).
// @Tags Public tags
// @Accept json
// @Produce json
// @Param code path string true "Tag code"
// @Param body body tagin.ScanRequest true "Visitor"
// @Success 201 {object} dto.ResponseDto{data=tagout.ScanReceiptResponse}
// @Failure 400 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Failure 429 {object} httpapi.Problem
// @Router /v1/public/tags/{code}/scans [post]
func (pc *PublicTagController) RecordScan() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req tagin.ScanRequest
		if !bindPublic(c, &req) {
			return
		}
		receipt, err := pc.tags.RecordScan(c.Request.Context(), c.Param("code"), req.VisitorID, pc.clientIP(c), c.Request.UserAgent())
		if err != nil {
			writePublicTagError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto.ResponseDto{Status: http.StatusCreated, Message: "Created", Data: tagout.ScanReceiptResponse{
			ScanID:    receipt.ScanID,
			ScanToken: receipt.Token,
		}})
	}
}

// SendMessage godoc
// @Summary Leave a message for the pet's family
// @Description No login; requires the X-Scan-Token of the scan. body 1-500 characters, finderName up to 80, finderPhone with the pet record phone rules. website is a honeypot: when filled the request is accepted and nothing is stored. Rate limited per IP (5/hour).
// @Tags Public tags
// @Accept json
// @Produce json
// @Param scanId path string true "Scan ID"
// @Param X-Scan-Token header string true "Token returned when the scan was registered"
// @Param body body tagin.FinderMessageRequest true "Message"
// @Success 202 {object} dto.ResponseDto
// @Failure 400 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Failure 429 {object} httpapi.Problem
// @Router /v1/public/scans/{scanId}/messages [post]
func (pc *PublicTagController) SendMessage() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req tagin.FinderMessageRequest
		if !bindPublic(c, &req) {
			return
		}
		if strings.TrimSpace(req.Website) == "" {
			err := pc.tags.SendMessage(c.Request.Context(), c.Param("scanId"), c.GetHeader(scanTokenHeader), domain.FinderMessageInput{
				Body:        req.Body,
				FinderName:  req.FinderName,
				FinderPhone: req.FinderPhone,
			})
			if err != nil {
				writePublicTagError(c, err)
				return
			}
		}
		// Honeypot hits get the same answer as a stored message.
		c.JSON(http.StatusAccepted, dto.ResponseDto{Status: http.StatusAccepted, Message: "Accepted"})
	}
}

// ShareLocation godoc
// @Summary Share where the pet was found
// @Description No login; requires the X-Scan-Token of the scan. The finder chooses to send the position of the device: lat -90..90, lng -180..180, accuracy in meters 0..100000, all required. Allowed once per scan (409 afterwards). The owner is notified. Rate limited per IP (10/hour).
// @Tags Public tags
// @Accept json
// @Param scanId path string true "Scan ID"
// @Param X-Scan-Token header string true "Token returned when the scan was registered"
// @Param body body tagin.ShareLocationRequest true "Position"
// @Success 204 "Location stored"
// @Failure 400 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Failure 409 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Failure 429 {object} httpapi.Problem
// @Router /v1/public/scans/{scanId}/location [put]
func (pc *PublicTagController) ShareLocation() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req tagin.ShareLocationRequest
		if !bindPublic(c, &req) {
			return
		}
		var missing []domain.PetFieldIssue
		for _, f := range []struct {
			name string
			nil  bool
		}{{"lat", req.Lat == nil}, {"lng", req.Lng == nil}, {"accuracy", req.Accuracy == nil}} {
			if f.nil {
				missing = append(missing, domain.PetFieldIssue{Field: f.name, Code: "required"})
			}
		}
		if len(missing) > 0 {
			writePublicTagError(c, &domain.PetValidationError{Issues: missing})
			return
		}
		if err := pc.tags.ShareLocation(c.Request.Context(), c.Param("scanId"), c.GetHeader(scanTokenHeader), *req.Lat, *req.Lng, *req.Accuracy); err != nil {
			writePublicTagError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// Contact godoc
// @Summary Reveal the family's phone
// @Description No login; requires the X-Scan-Token of the scan. Returns only the main owner phone and a WhatsApp link, and only when the owner shows the contact button; otherwise 404. This is the only public route that returns a phone. Rate limited per IP (3/hour).
// @Tags Public tags
// @Produce json
// @Param scanId path string true "Scan ID"
// @Param X-Scan-Token header string true "Token returned when the scan was registered"
// @Success 200 {object} dto.ResponseDto{data=tagout.PublicContactResponse}
// @Failure 404 {object} httpapi.Problem
// @Failure 429 {object} httpapi.Problem
// @Router /v1/public/scans/{scanId}/contact [post]
func (pc *PublicTagController) Contact() gin.HandlerFunc {
	return func(c *gin.Context) {
		contact, err := pc.tags.RevealContact(c.Request.Context(), c.Param("scanId"), c.GetHeader(scanTokenHeader))
		if err != nil {
			writePublicTagError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: tagout.PublicContactResponse{
			Phone:       contact.Phone,
			WhatsAppURL: contact.WhatsAppURL,
		}})
	}
}

// bindPublic decodes a small JSON body or writes the problem response.
func bindPublic(c *gin.Context, dst any) bool {
	if err := httpapi.BindJSONLimited(c, dst, publicMaxBodyBytes); err != nil {
		httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
		return false
	}
	return true
}

// writePublicTagError never reveals whether a tag, scan or token exists: all of
// them are the same 404. Unknown errors are a redacted 500.
func writePublicTagError(c *gin.Context, err error) {
	requestID := httpapi.RequestID(c)
	var invalid *domain.PetValidationError
	switch {
	case errors.As(err, &invalid):
		httpapi.WriteProblem(c, httpapi.Validation("Request contains invalid fields", requestID, fieldErrors(invalid)))
	case errors.Is(err, domain.ErrLocationAlreadyShared):
		httpapi.WriteProblem(c, httpapi.Conflict(publicLocationSharedDetail, requestID))
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrInvalidScanToken):
		httpapi.WriteProblem(c, httpapi.NotFound(publicNotFoundDetail, requestID))
	default:
		httpapi.WriteProblem(c, httpapi.Internal(requestID))
	}
}

func fieldErrors(invalid *domain.PetValidationError) []httpapi.FieldError {
	fields := make([]httpapi.FieldError, 0, len(invalid.Issues))
	for _, issue := range invalid.Issues {
		fields = append(fields, httpapi.FieldError{Field: issue.Field, Code: issue.Code})
	}
	return fields
}

// toPublicTagViewResponse maps the projection field by field: anything not
// copied here can never reach the public JSON.
func toPublicTagViewResponse(code string, view domain.PublicTagView) tagout.PublicTagViewResponse {
	out := tagout.PublicTagViewResponse{State: string(view.State)}
	p := view.Profile
	if p == nil {
		return out
	}
	profile := &tagout.PublicProfileResponse{
		Name:                 p.Name,
		Species:              p.Species,
		Breed:                p.Breed,
		Color:                p.Color,
		Sex:                  p.Sex,
		Size:                 p.Size,
		AgeMonths:            p.AgeMonths,
		DistinctiveFeatures:  p.DistinctiveFeatures,
		Allergies:            p.Allergies,
		SpecialConditions:    p.SpecialConditions,
		Surgeries:            p.Surgeries,
		Temperament:          p.Temperament,
		OwnerFirstName:       p.OwnerFirstName,
		SecondOwnerFirstName: p.SecondOwnerFirstName,
		Comuna:               p.Comuna,
		CanContact:           p.CanContact,
		Lost:                 p.Lost,
	}
	if p.HasPhoto {
		profile.PhotoURL = fmt.Sprintf("/v1/public/tags/%s/photo?v=%d", code, p.PhotoVersion)
	}
	out.Profile = profile
	return out
}
