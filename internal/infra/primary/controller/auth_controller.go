package controller

import (
	authBaseCore "github.com/WilsonSayago/authBase/core"
	"github.com/WilsonSayago/initModules"
	"github.com/gin-gonic/gin"
	"jwt/base/internal/core"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/infra/config"
	"jwt/base/internal/infra/primary/controller/dto"
	"jwt/base/internal/infra/primary/controller/dto/inputs/auth"
	"net/http"
)

type AuthController struct {
	authenticationService authBaseCore.AuthenticationUseCase
	authorizationService  authBaseCore.AuthorizationUseCase[domain.User, *gin.Context]
	userService           core.UserUseCase
	r                     *gin.Engine
	t                     config.ITranslate
}

func GetAuthControllerInstance(
	authenticationService authBaseCore.AuthenticationUseCase,
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *gin.Context],
	userService core.UserUseCase,
	r *gin.Engine,
	t config.ITranslate) *AuthController {
	instance := initModules.GetInstance("AuthController", func() interface{} {
		return &AuthController{
			authenticationService: authenticationService,
			authorizationService:  authorizationService,
			userService:           userService,
			r:                     r,
			t:                     t,
		}
	})
	return instance.(*AuthController)
}

func (authC *AuthController) Start() {
	authC.RunController()
}

func (authC *AuthController) RunController() {

	// Protected routes group
	protected := (*authC.r).Group("/v1/auth")
	{
		protected.POST("/login", authC.Login())
		protected.POST("/refresh-token", authC.RefreshToken())
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
func (authC *AuthController) Login() func(c *gin.Context) {
	return func(c *gin.Context) {
		var loginData auth.LoginRequestDto
		if !handleBind(authC.t, c, &loginData) {
			return
		}
		token, refreshToken, err := authC.authenticationService.Login(loginData.Email, loginData.Password)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
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
// @Description Refreshes an expired access token using a refresh token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param refreshToken body auth.RefreshTokenRequestDto true "Refresh token"
// @Success 200 {object} dto.AuthDto "New tokens"
// @Failure 400 {object} dto.ResponseDto "Invalid request"
// @Router /v1/auth/refresh-token [post]
func (authC *AuthController) RefreshToken() func(c *gin.Context) {
	return func(c *gin.Context) {
		var refreshToken auth.RefreshTokenRequestDto
		if !handleBind(authC.t, c, &refreshToken) {
			return
		}
		token, refreshTokenNew, err := authC.authenticationService.RefreshToken(refreshToken.RefreshToken)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
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
