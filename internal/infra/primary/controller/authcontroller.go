package controller

import (
	"fmt"
	authBaseCore "github.com/WilsonSayago/authBase/core"
	"github.com/WilsonSayago/initModules"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"io"
	"jwt/base/internal/core"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/infra/primary/controller/dto"
	"net/http"
)

type AuthController struct {
	authenticationService authBaseCore.AuthenticationUseCase
	authorizationService  authBaseCore.AuthorizationUseCase[domain.User, *gin.Context]
	userService           core.UserUseCase
	r                     *gin.Engine
}

func GetAuthControllerInstance(
	authenticationService authBaseCore.AuthenticationUseCase,
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *gin.Context],
	userService core.UserUseCase,
	r *gin.Engine) *AuthController {
	instance := initModules.GetInstance("AuthController", func() interface{} {
		return &AuthController{
			authenticationService: authenticationService,
			authorizationService:  authorizationService,
			userService:           userService,
			r:                     r,
		}
	})
	return instance.(*AuthController)
}

func (authC *AuthController) Start() {
	authC.RunController()
}

func (authC *AuthController) RunController() {

	// Protected routes group
	protected := (*authC.r).Group("/auth")
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
		if err := c.ShouldBindJSON(&loginData); err != nil {
			if err == io.EOF {
				c.JSON(http.StatusBadRequest, dto.ResponseDto{
					Status:  http.StatusBadRequest,
					Message: "Error login - no data provided",
					Data:    nil,
				})
				return
			}

			var fieldErrors []string
			for _, fieldErr := range err.(validator.ValidationErrors) {
				if fieldErr.Field() == "Email" && fieldErr.Tag() == "email" {
					fieldErrors = append(fieldErrors, "Email is not a valid email address")
				} else {
					fieldErrors = append(fieldErrors, fmt.Sprintf("%s is %s", fieldErr.Field(), fieldErr.Tag()))
				}
			}
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: "Error login - invalid data",
				Data:    fieldErrors,
			})
			return
		}
		token, refreshToken, err := authC.authenticationService.Login(loginData.Email, loginData.Password)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: "Error login - invalid data",
				Data:    nil,
			})
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "ok",
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
			RefreshToken string `json:"refreshToken"`
		}
		if err := c.ShouldBindJSON(&refreshToken); err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: "Error refresh token - invalid data",
				Data:    nil,
			})
			return
		}
		token, refreshTokenNew, err := authC.authenticationService.RefreshToken(refreshToken.RefreshToken)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: "Error refresh token - invalid data",
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
