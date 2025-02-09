package controller

import (
	authBaseCore "github.com/WilsonSayago/authBase/core"
	"github.com/WilsonSayago/initModules"
	"github.com/gin-gonic/gin"
	"jwt/base/internal/core"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/infra/config"
	"jwt/base/internal/infra/primary/controller/dto"
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

func (authC *AuthController) Login() func(c *gin.Context) {
	return func(c *gin.Context) {
		var loginData struct {
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required"`
		}
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
				Token:        token,
				RefreshToken: refreshToken,
			},
		})
	}
}

func (authC *AuthController) RefreshToken() func(c *gin.Context) {
	return func(c *gin.Context) {
		var refreshToken struct {
			RefreshToken string `json:"refreshToken" binding:"required"`
		}
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
				Token:        token,
				RefreshToken: refreshTokenNew,
			},
		})
	}
}
