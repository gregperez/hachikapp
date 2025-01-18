package controller

import (
	authBaseCore "github.com/WilsonSayago/authBase/core"
	authBaseDomain "github.com/WilsonSayago/authBase/core/domains"
	"github.com/WilsonSayago/initModules"
	"github.com/gin-gonic/gin"
	"jwt/base/internal/core"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/infra/primary/controller/dto"
	"net/http"
)

type UserController struct {
	authenticationService authBaseCore.AuthenticationUseCase
	authorizationService  authBaseCore.AuthorizationUseCase[domain.User, *gin.Context]
	userService           core.IUserUseCase
	r                     *gin.Engine
}

func GetUserControllerInstance(
	authenticationService authBaseCore.AuthenticationUseCase,
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *gin.Context],
	userService core.IUserUseCase,
	r *gin.Engine) *UserController {
	instance := initModules.GetInstance("UserController", func() interface{} {
		return &UserController{
			authenticationService: authenticationService,
			authorizationService:  authorizationService,
			userService:           userService,
			r:                     r,
		}
	})
	return instance.(*UserController)
}

func (userC *UserController) Start() {
	userC.RunController()
}

func (userC *UserController) RunController() {
	(*userC.r).POST("/login", func(c *gin.Context) {
		var loginData struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&loginData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		token, err := userC.authenticationService.Login(loginData.Email, loginData.Password)
		if err != nil {
			c.JSON(http.StatusBadRequest, nil)
			return
		}
		c.JSON(http.StatusOK, token)
	})

	// Protected routes group
	protected := (*userC.r).Group("/users")
	protected.Use(userC.authorizationService.AuthorizeJWT())
	{
		protected.GET("/",
			userC.authorizationService.PoliciesGuard(userC.GetAll(), nil,
				"USERS", authBaseDomain.READ))
	}
}

func (userC *UserController) GetAll() func(c *gin.Context) {
	return func(c *gin.Context) {
		users, err := userC.userService.FindAll()
		if err != nil {
			c.JSON(http.StatusBadRequest, nil)
		}

		list := make([]dto.UserDto, 0)
		for _, user := range users {
			list = append(list, dto.UserDto{Email: user.GetEmail(), IsAdmin: user.GetIsAdmin()})
		}
		c.JSON(http.StatusOK, list)
	}
}
