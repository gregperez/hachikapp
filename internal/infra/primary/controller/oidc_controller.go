package controller

import (
	"net/http"
	"strings"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/core/services"
	"github.com/gregperez/hachikapp-backend/internal/infra/config"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/appconfig"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/inputs/auth"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/ginadapter"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
)

const googleProvider = appconfig.OIDCProviderGoogle

type OIDCController struct {
	service        *services.OIDCService
	authorization  authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	limiter        ports.LoginLimiter
	originResolver *ClientOriginResolver
	cfg            appconfig.OIDCConfig
	secureCookie   bool
	r              *gin.Engine
	t              config.ITranslate
}

func NewOIDCController(
	service *services.OIDCService,
	authorization authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	r *gin.Engine,
	t config.ITranslate,
	limiter ports.LoginLimiter,
	originResolver *ClientOriginResolver,
	cfg appconfig.OIDCConfig,
	secureCookie bool,
) *OIDCController {
	return &OIDCController{
		service:        service,
		authorization:  authorization,
		limiter:        limiter,
		originResolver: originResolver,
		cfg:            cfg,
		secureCookie:   secureCookie,
		r:              r,
		t:              t,
	}
}

func (oc *OIDCController) RunController() {
	group := oc.r.Group("/v1/auth/oidc/google")
	group.GET("/login", oc.Login())
	group.GET("/callback", oc.Callback())
	group.POST("/exchange", oc.Exchange())
	if oc.authorization != nil {
		group.POST("/link", ginadapter.Middleware(oc.authorization.AuthorizeJWT()), oc.Link())
		group.DELETE("", ginadapter.Middleware(oc.authorization.AuthorizeJWT()), oc.Unlink())
	}
}

// Login godoc
// @Summary Start Google OIDC login
// @Description Redirects the browser to Google using Authorization Code + PKCE. Tokens never appear in the redirect.
// @Tags Authentication
// @Produce json
// @Param return_to query string false "Exact allowlisted frontend destination"
// @Success 302 "Redirect to the identity provider"
// @Failure 400 {object} httpapi.Problem "Destination is not allowlisted"
// @Failure 429 {object} httpapi.Problem "Authentication rate limit exceeded"
// @Failure 503 {object} httpapi.Problem "OIDC is disabled or unavailable"
// @Router /v1/auth/oidc/google/login [get]
func (oc *OIDCController) Login() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !oc.allowOrigin(c) || !oc.ready(c) {
			return
		}
		start, err := oc.service.StartLogin(c.Request.Context(), googleProvider, c.Query("return_to"))
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		oc.writeStateCookie(c, start)
		c.Redirect(http.StatusFound, start.AuthorizationURL)
	}
}

// Link godoc
// @Summary Start explicit Google account linking
// @Description Requires a current access token and password confirmation. Does not auto-link by email.
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body auth.OIDCLinkRequestDto true "Password confirmation and allowlisted destination"
// @Success 302 "Redirect to the identity provider"
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 429 {object} httpapi.Problem
// @Failure 503 {object} httpapi.Problem
// @Router /v1/auth/oidc/google/link [post]
func (oc *OIDCController) Link() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !oc.allowOrigin(c) || !oc.ready(c) {
			return
		}
		actor, ok := requireIdentity(oc.authorization, c)
		if !ok {
			return
		}
		var body auth.OIDCLinkRequestDto
		if !handleBind(oc.t, c, &body) {
			return
		}
		start, err := oc.service.StartLink(c.Request.Context(), googleProvider, actor.GetId(), body.Password, body.ReturnTo)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		oc.writeStateCookie(c, start)
		c.Redirect(http.StatusFound, start.AuthorizationURL)
	}
}

// Callback godoc
// @Summary Complete Google OIDC callback
// @Description Consumes the one-time state cookie and code, then redirects to an allowlisted destination with an opaque grant or link status.
// @Tags Authentication
// @Produce json
// @Param code query string false "Authorization code"
// @Param state query string true "CSRF state"
// @Success 302 "Redirect to the allowlisted destination"
// @Failure 401 {object} httpapi.Problem
// @Failure 409 {object} httpapi.Problem
// @Failure 429 {object} httpapi.Problem
// @Failure 503 {object} httpapi.Problem
// @Router /v1/auth/oidc/google/callback [get]
func (oc *OIDCController) Callback() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !oc.allowOrigin(c) || !oc.ready(c) {
			return
		}
		cookie, _ := c.Request.Cookie(oc.cfg.CookieName)
		cookieState := ""
		if cookie != nil {
			cookieState = cookie.Value
		}
		result, err := oc.service.FinishCallback(
			c.Request.Context(),
			googleProvider,
			cookieState,
			c.Query("state"),
			c.Query("code"),
		)
		oc.clearStateCookie(c)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		if strings.Contains(strings.ToLower(result.RedirectURL), "eyj") {
			httpapi.WriteProblem(c, httpapi.Internal(httpapi.RequestID(c)))
			return
		}
		c.Redirect(http.StatusFound, result.RedirectURL)
	}
}

// Exchange godoc
// @Summary Exchange an OIDC grant for local tokens
// @Description Consumes a one-time opaque grant and returns the existing local token contract. The grant is not a JWT.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param body body auth.OIDCExchangeRequestDto true "Opaque grant"
// @Success 200 {object} dto.ResponseDto{data=dto.AuthDto}
// @Failure 401 {object} httpapi.Problem
// @Failure 429 {object} httpapi.Problem
// @Failure 503 {object} httpapi.Problem
// @Router /v1/auth/oidc/google/exchange [post]
func (oc *OIDCController) Exchange() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !oc.allowOrigin(c) || !oc.ready(c) {
			return
		}
		var body auth.OIDCExchangeRequestDto
		if !handleBind(oc.t, c, &body) {
			return
		}
		attachSessionMeta(c, oc.originResolver)
		access, refresh, err := oc.service.ExchangeGrant(c.Request.Context(), body.Grant)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data: dto.AuthDto{
				AccessToken:  access,
				RefreshToken: refresh,
			},
		})
	}
}

// Unlink godoc
// @Summary Unlink a Google identity
// @Description Removes the explicit (issuer, subject) link after password confirmation. Password login remains available.
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body auth.OIDCUnlinkRequestDto true "Password confirmation"
// @Success 204 "Unlinked"
// @Failure 401 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Failure 429 {object} httpapi.Problem
// @Router /v1/auth/oidc/google [delete]
func (oc *OIDCController) Unlink() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !oc.allowOrigin(c) || !oc.ready(c) {
			return
		}
		actor, ok := requireIdentity(oc.authorization, c)
		if !ok {
			return
		}
		var body auth.OIDCUnlinkRequestDto
		if !handleBind(oc.t, c, &body) {
			return
		}
		if err := oc.service.Unlink(c.Request.Context(), googleProvider, actor.GetId(), body.Password); err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func (oc *OIDCController) ready(c *gin.Context) bool {
	if oc.service == nil {
		httpapi.WriteProblem(c, httpapi.ServiceUnavailable("Federated login is unavailable", httpapi.RequestID(c)))
		return false
	}
	return true
}

func (oc *OIDCController) allowOrigin(c *gin.Context) bool {
	if oc.limiter == nil {
		return true
	}
	origin := "unknown"
	if oc.originResolver != nil {
		origin = oc.originResolver.Origin(c.Request)
	}
	auth := &AuthController{limiter: oc.limiter, originResolver: oc.originResolver}
	return auth.allow(c, ports.LoginLimitOrigin, authenticationLimitKey("oidc:"+origin))
}

func (oc *OIDCController) writeStateCookie(c *gin.Context, start services.OIDCStart) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oc.cfg.CookieName,
		Value:    start.State,
		Path:     oc.cfg.CookiePath,
		MaxAge:   int(start.CookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   oc.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (oc *OIDCController) clearStateCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oc.cfg.CookieName,
		Value:    "",
		Path:     oc.cfg.CookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   oc.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}
