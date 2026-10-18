package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/authz"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/config"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto"
	settingsdto "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/outputs/settings"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/ginadapter"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	"github.com/gin-gonic/gin"
)

type SettingsController struct {
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	settings      core.SettingsUseCase
	r             *gin.Engine
	t             config.ITranslate
}

func NewSettingsController(
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	settings core.SettingsUseCase,
	r *gin.Engine,
	t config.ITranslate,
) *SettingsController {
	return &SettingsController{authorization: authorization, settings: settings, r: r, t: t}
}

func (sc *SettingsController) RunController() {
	if sc.settings == nil {
		return
	}
	sc.r.GET("/v1/settings/public", sc.Public())
	protected := sc.r.Group("/v1/settings")
	protected.Use(ginadapter.Middleware(sc.authorization.AuthorizeJWT()))
	{
		protected.GET("", ginadapter.Middleware(sc.authorization.PoliciesGuard(
			ginadapter.Handler(sc.Get()), nil, authz.EntitySettings, authz.Read)))
		protected.PUT("", ginadapter.Middleware(sc.authorization.PoliciesGuard(
			ginadapter.Handler(sc.Update()), nil, authz.EntitySettings, authz.Update)))
	}
}

// Public godoc
// @Summary Get public application branding
// @Description Returns display name, logo URL and default locale. No authentication.
// @Tags Settings
// @Produce json
// @Success 200 {object} dto.ResponseDto{data=settingsdto.PublicSettingsResponse}
// @Failure 500 {object} httpapi.Problem
// @Router /v1/settings/public [get]
func (sc *SettingsController) Public() gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := sc.settings.Get(c.Request.Context())
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		public := domain.PublicView(settings)
		writeSettingsCache(c, settings.Version, true)
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data: settingsdto.PublicSettingsResponse{
				DisplayName:   public.DisplayName,
				LogoURL:       public.LogoURL,
				DefaultLocale: public.DefaultLocale,
			},
		})
	}
}

// Get godoc
// @Summary Get administrative application settings
// @Description Returns typed settings including optimistic version. Requires SETTINGS.read.
// @Tags Settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.ResponseDto{data=settingsdto.SettingsResponse}
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Router /v1/settings [get]
func (sc *SettingsController) Get() gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := sc.settings.Get(c.Request.Context())
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		writeSettingsCache(c, settings.Version, false)
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    toSettingsResponse(settings),
		})
	}
}

// Update godoc
// @Summary Update application settings
// @Description Allowlisted branding fields with optimistic version. Unknown or protected keys return 422. Requires SETTINGS.update.
// @Tags Settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param If-Match header string false "Expected version as a quoted ETag"
// @Param body body settingsdto.UpdateSettingsRequest true "Allowlisted settings"
// @Success 200 {object} dto.ResponseDto{data=settingsdto.SettingsResponse}
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Failure 409 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Router /v1/settings [put]
func (sc *SettingsController) Update() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(sc.authorization, c)
		if !ok {
			return
		}
		patch, err := decodeSettingsPatch(c)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		if match := strings.TrimSpace(c.GetHeader("If-Match")); match != "" {
			etagVersion, err := parseSettingsETag(match)
			if err != nil || etagVersion != patch.Version {
				httpapi.WriteProblem(c, httpapi.Validation("If-Match does not match the request version", httpapi.RequestID(c), []httpapi.FieldError{{Field: "version", Code: "mismatch"}}))
				return
			}
		}
		saved, err := sc.settings.Update(c.Request.Context(), actor.GetId(), patch)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		writeSettingsCache(c, saved.Version, false)
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    toSettingsResponse(saved),
		})
	}
}

func decodeSettingsPatch(c *gin.Context) (domain.AppSettingsPatch, error) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return domain.AppSettingsPatch{}, httpapi.ErrEmptyBody
	}
	body := http.MaxBytesReader(c.Writer, c.Request.Body, httpapi.DefaultMaxBodyBytes)
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		return domain.AppSettingsPatch{}, httpapi.ErrInvalidJSON
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return domain.AppSettingsPatch{}, httpapi.ErrEmptyBody
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return domain.AppSettingsPatch{}, httpapi.ErrInvalidJSON
	}
	fields := make([]httpapi.FieldError, 0)
	for key := range keys {
		if domain.IsAllowedSettingKey(key) {
			continue
		}
		code := "unknown"
		if domain.IsProtectedSettingKey(key) {
			code = "protected"
		}
		fields = append(fields, httpapi.FieldError{Field: key, Code: code})
	}
	if len(fields) > 0 {
		return domain.AppSettingsPatch{}, &httpapi.ValidationError{Fields: fields}
	}
	var req settingsdto.UpdateSettingsRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return domain.AppSettingsPatch{}, httpapi.ErrInvalidJSON
	}
	return domain.AppSettingsPatch{
		Version:       req.Version,
		DisplayName:   req.DisplayName,
		LogoURL:       req.LogoURL,
		DefaultLocale: req.DefaultLocale,
	}, nil
}

func toSettingsResponse(settings domain.AppSettings) settingsdto.SettingsResponse {
	updatedAt := ""
	if !settings.UpdatedAt.IsZero() {
		updatedAt = settings.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return settingsdto.SettingsResponse{
		DisplayName:   settings.DisplayName,
		LogoURL:       settings.LogoURL,
		DefaultLocale: settings.DefaultLocale,
		Version:       settings.Version,
		UpdatedAt:     updatedAt,
		UpdatedBy:     settings.UpdatedBy,
	}
}

func writeSettingsCache(c *gin.Context, version int64, public bool) {
	c.Header("ETag", fmt.Sprintf(`"%d"`, version))
	if public {
		c.Header("Cache-Control", "public, max-age=60")
		return
	}
	c.Header("Cache-Control", "private, no-store")
}

func parseSettingsETag(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, `"`)
	if value == "" {
		return 0, fmt.Errorf("empty etag")
	}
	return strconv.ParseInt(value, 10, 64)
}
