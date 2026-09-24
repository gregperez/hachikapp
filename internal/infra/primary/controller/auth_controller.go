package controller

import (
	"errors"
	"net/http"

	"github.com/WilsonSayago/base-golang/internal/core"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto/inputs/auth"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/ginadapter"

	authBaseCore "github.com/WilsonSayago/authBase/v3/core"
	authBaseServices "github.com/WilsonSayago/authBase/v3/core/services"
	initModules "github.com/WilsonSayago/initModules/v2"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	authenticationService authBaseCore.AuthenticationUseCase
	authorizationService  authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	userService           core.UserUseCase
	tokens                *authBaseServices.TokenManager
	r                     *gin.Engine
	t                     config.ITranslate
}

func GetAuthControllerInstance(
	authenticationService authBaseCore.AuthenticationUseCase,
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	userService core.UserUseCase,
	tokens *authBaseServices.TokenManager,
	r *gin.Engine,
	t config.ITranslate,
) *AuthController {
	return initModules.Once(func() *AuthController {
		return &AuthController{
			authenticationService: authenticationService,
			authorizationService:  authorizationService,
			userService:           userService,
			tokens:                tokens,
			r:                     r,
			t:                     t,
		}
	})
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
// @Description Autentica a un usuario y devuelve tokens JWT
// @Tags Authentication
// @Accept json
// @Produce json
// @Param loginData body auth.LoginRequestDto true "Credenciales de inicio de sesión"
// @Success 200 {object} dto.AuthDto "Tokens de acceso y refresh"
// @Failure 400 {object} dto.ResponseDto "Error en la petición"
// @Failure 401 {object} dto.ResponseDto "Credenciales incorrectas"
// @Router /v1/auth/login [post]
func (authC *AuthController) Login() gin.HandlerFunc {
	return func(c *gin.Context) {
		var loginData auth.LoginRequestDto
		if !handleBind(authC.t, c, &loginData) {
			return
		}
		token, refreshToken, err := authC.authenticationService.Login(c.Request.Context(), loginData.Email, loginData.Password)
		if err != nil {
			status := http.StatusUnauthorized
			if !errors.Is(err, authBaseCore.ErrInvalidCredentials) {
				status = http.StatusBadRequest
			}
			c.JSON(status, dto.ResponseDto{
				Status:  status,
				Message: authC.t.TranslateByLang(c.GetHeader("lang"), "errors.token-login", nil),
				Data:    nil,
			})
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
// @Success 200 {object} dto.AuthDto "New tokens"
// @Failure 401 {object} dto.ResponseDto "Invalid refresh"
// @Router /v1/auth/refresh-token [post]
func (authC *AuthController) RefreshToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body auth.RefreshTokenRequestDto
		if !handleBind(authC.t, c, &body) {
			return
		}
		token, refreshTokenNew, err := authC.authenticationService.RefreshToken(c.Request.Context(), body.RefreshToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, dto.ResponseDto{
				Status:  http.StatusUnauthorized,
				Message: authC.t.TranslateByLang(c.GetHeader("lang"), "errors.refresh-token", nil),
				Data:    nil,
			})
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "ok",
			Data: dto.AuthDto{
				AccessToken:  token,
				RefreshToken: refreshTokenNew,
			},
		})
	}
}

// Logout godoc
// @Summary Logout / revoke refresh family
// @Description Revokes the refresh token family. Idempotent.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param refreshToken body auth.RefreshTokenRequestDto true "Refresh token"
// @Success 204 "Revoked"
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
		_ = authC.authenticationService.RevokeRefreshFamily(c.Request.Context(), claims.FamilyID)
		c.Status(http.StatusNoContent)
	}
}
