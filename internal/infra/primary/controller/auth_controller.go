package controller

import (
	"net/http"

	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/config"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/inputs/auth"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/ginadapter"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseServices "github.com/WilsonSayago/authBase/v4/core/services"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	authenticationService authBaseCore.AuthenticationUseCase
	authorizationService  authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	userService           core.UserUseCase
	tokens                *authBaseServices.TokenManager
	r                     *gin.Engine
	t                     config.ITranslate
	limiter               ports.LoginLimiter
	originResolver        *ClientOriginResolver
}

func NewAuthController(
	authenticationService authBaseCore.AuthenticationUseCase,
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	userService core.UserUseCase,
	tokens *authBaseServices.TokenManager,
	r *gin.Engine,
	t config.ITranslate,
	limiter ports.LoginLimiter,
	originResolver *ClientOriginResolver,
) *AuthController {
	return &AuthController{
		authenticationService: authenticationService,
		authorizationService:  authorizationService,
		userService:           userService,
		tokens:                tokens,
		r:                     r,
		t:                     t,
		limiter:               limiter,
		originResolver:        originResolver,
	}
}

// GetAuthControllerInstance is an alias for NewAuthController.
//
// Deprecated: use NewAuthController.
func GetAuthControllerInstance(
	authenticationService authBaseCore.AuthenticationUseCase,
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	userService core.UserUseCase,
	tokens *authBaseServices.TokenManager,
	r *gin.Engine,
	t config.ITranslate,
	limiter ports.LoginLimiter,
	originResolver *ClientOriginResolver,
) *AuthController {
	return NewAuthController(
		authenticationService,
		authorizationService,
		userService,
		tokens,
		r,
		t,
		limiter,
		originResolver,
	)
}

func (authC *AuthController) RunController() {
	group := authC.r.Group("/v1/auth")
	{
		group.POST("/login", authC.Login())
		group.POST("/refresh-token", authC.RefreshToken())
		group.POST("/logout", authC.Logout())
	}
}

// Login godoc
// @Summary User login
// @Description Authenticates a user and returns JWT access and refresh tokens
// @Tags Authentication
// @Accept json
// @Produce json
// @Param loginData body auth.LoginRequestDto true "Login credentials"
// @Success 200 {object} dto.ResponseDto{data=dto.AuthDto} "Tokens"
// @Failure 400 {object} httpapi.Problem "Malformed JSON or empty body"
// @Failure 401 {object} httpapi.Problem "Invalid credentials"
// @Failure 429 {object} httpapi.Problem "Authentication rate limit exceeded"
// @Failure 422 {object} httpapi.Problem "Validation failed"
// @Router /v1/auth/login [post]
func (authC *AuthController) Login() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !authC.allowOrigin(c) {
			return
		}
		var loginData auth.LoginRequestDto
		if !handleBind(authC.t, c, &loginData) {
			return
		}
		accountKey := authenticationLimitKey(loginData.Email)
		if !authC.allow(c, ports.LoginLimitAccount, accountKey) {
			return
		}
		attachSessionMeta(c, authC.originResolver)
		token, refreshToken, err := authC.authenticationService.Login(c.Request.Context(), loginData.Email, loginData.Password)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		if !authC.reset(c, ports.LoginLimitAccount, accountKey) {
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data: dto.AuthDto{
				AccessToken:  token,
				RefreshToken: refreshToken,
			},
		})
	}
}

// RefreshToken godoc
// @Summary Refresh JWT token
// @Description Refreshes tokens using a refresh token (single-use rotation)
// @Tags Authentication
// @Accept json
// @Produce json
// @Param refreshToken body auth.RefreshTokenRequestDto true "Refresh token"
// @Success 200 {object} dto.ResponseDto{data=dto.AuthDto} "New tokens"
// @Failure 401 {object} httpapi.Problem "Invalid refresh"
// @Failure 429 {object} httpapi.Problem "Authentication rate limit exceeded"
// @Failure 422 {object} httpapi.Problem "Validation failed"
// @Router /v1/auth/refresh-token [post]
func (authC *AuthController) RefreshToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !authC.allowOrigin(c) {
			return
		}
		var body auth.RefreshTokenRequestDto
		if !handleBind(authC.t, c, &body) {
			return
		}
		var familyKey string
		if claims, err := authC.tokens.ParseRefresh(body.RefreshToken); err == nil {
			familyKey = authenticationLimitKey(claims.FamilyID)
			if !authC.allow(c, ports.RefreshLimitFamily, familyKey) {
				return
			}
		}
		attachSessionMeta(c, authC.originResolver)
		token, refreshTokenNew, err := authC.authenticationService.RefreshToken(c.Request.Context(), body.RefreshToken)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		if familyKey != "" && !authC.reset(c, ports.RefreshLimitFamily, familyKey) {
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data: dto.AuthDto{
				AccessToken:  token,
				RefreshToken: refreshTokenNew,
			},
		})
	}
}

// Logout godoc
// @Summary Logout / revoke refresh family
// @Description Revokes the refresh token family. Idempotent for invalid tokens.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param refreshToken body auth.RefreshTokenRequestDto true "Refresh token"
// @Success 204 "Revoked"
// @Failure 422 {object} httpapi.Problem "Validation failed"
// @Router /v1/auth/logout [post]
func (authC *AuthController) Logout() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body auth.RefreshTokenRequestDto
		if !handleBind(authC.t, c, &body) {
			return
		}
		claims, err := authC.tokens.ParseRefresh(body.RefreshToken)
		if err != nil {
			// Idempotent public outcome: treat invalid token as already logged out.
			c.Status(http.StatusNoContent)
			return
		}
		if err := authC.authenticationService.RevokeRefreshFamily(c.Request.Context(), claims.FamilyID); err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.Status(http.StatusNoContent)
	}
}
