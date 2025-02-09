package controller

import (
	authBaseCore "github.com/WilsonSayago/authBase/core"
	authBaseDomain "github.com/WilsonSayago/authBase/core/domain"
	"github.com/WilsonSayago/initModules"
	"github.com/gin-gonic/gin"
	"jwt/base/internal/core"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/infra/config"
	"jwt/base/internal/infra/primary/controller/dto"
	"jwt/base/internal/infra/primary/controller/dto/inputs/users"
	users2 "jwt/base/internal/infra/primary/controller/dto/outputs/users"
	"net/http"
)

type UserController struct {
	authenticationService authBaseCore.AuthenticationUseCase
	authorizationService  authBaseCore.AuthorizationUseCase[domain.User, *gin.Context]
	userService           core.UserUseCase
	r                     *gin.Engine
	t                     config.ITranslate
}

func GetUserControllerInstance(
	authenticationService authBaseCore.AuthenticationUseCase,
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *gin.Context],
	userService core.UserUseCase,
	r *gin.Engine,
	t config.ITranslate) *UserController {
	instance := initModules.GetInstance("UserController", func() interface{} {
		return &UserController{
			authenticationService: authenticationService,
			authorizationService:  authorizationService,
			userService:           userService,
			r:                     r,
			t:                     t,
		}
	})
	return instance.(*UserController)
}

func (userC *UserController) Start() {
	userC.RunController()
}

func (userC *UserController) RunController() {
	// Protected routes group
	protected := (*userC.r).Group("/v1/users")
	protected.Use(userC.authorizationService.AuthorizeJWT())
	{
		protected.GET("/",
			userC.authorizationService.PoliciesGuard(userC.GetAll(), nil,
				"USERS", authBaseDomain.READ))
		protected.POST("/",
			userC.authorizationService.PoliciesGuard(userC.Create(), nil,
				"USERS", authBaseDomain.CREATE))
		protected.GET("/:id",
			userC.authorizationService.PoliciesGuard(userC.GetById(), nil,
				"USERS", authBaseDomain.READ))
		protected.PUT("/change-status/:id",
			userC.authorizationService.PoliciesGuard(userC.ChangeStatus(), nil,
				"USERS", authBaseDomain.UPDATE))
		protected.PUT("/:id",
			userC.authorizationService.PoliciesGuard(userC.Update(), nil,
				"USERS", authBaseDomain.UPDATE))
	}
}

func (userC *UserController) GetAll() func(c *gin.Context) {
	return func(c *gin.Context) {
		page, pageSize, offset := handlePagination(c)
		usersDb, total, err := userC.userService.FindAll(pageSize, offset)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: userC.t.TranslateByLang(c.GetHeader("lang"), "errors.users.get-all", nil),
				Data:    nil,
			})
			return
		}

		list := make([]users2.UserResponse, 0)
		for _, user := range usersDb {
			list = append(list, users2.UserResponse{
				BaseDto: dto.BaseDto{
					Id:     user.GetId(),
					Active: user.Active,
				},
				Email:   user.GetEmail(),
				IsAdmin: user.GetIsAdmin()})
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data: users2.UsersPageResponse{
				Page:       page,
				PageSize:   pageSize,
				TotalItems: total,
				TotalPages: (total + pageSize - 1) / pageSize,
				Data:       list,
			},
		})
	}
}

func (userC *UserController) Create() func(c *gin.Context) {
	return func(c *gin.Context) {
		var userDto users.UseDto
		if !handleBind(userC.t, c, &userDto) {
			return
		}
		user := domain.User{
			UserGeneric: authBaseDomain.NewUserGeneric(
				"",
				userDto.Name,
				userDto.Email,
				userDto.Password,
				nil,
				userDto.IsAdmin,
				false,
			),
		}
		err := userC.userService.Create(user)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: userC.t.TranslateByLang(c.GetHeader("lang"), "errors.users.save", nil),
				Data:    nil,
			})
		} else {
			c.JSON(http.StatusOK, dto.ResponseDto{
				Status:  http.StatusOK,
				Message: "Ok",
				Data:    nil,
			})
		}
	}
}

func (userC *UserController) GetById() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		user, err := userC.userService.FindById(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: userC.t.TranslateByLang(c.GetHeader("lang"), "errors.users.get-by-id", nil),
				Data:    nil,
			})
			return
		}

		userDto := users.UseDto{
			BaseDto: dto.BaseDto{
				Id:     user.GetId(),
				Active: user.Active,
			},
			Name:    user.GetName(),
			Email:   user.GetEmail(),
			IsAdmin: user.GetIsAdmin(),
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    userDto,
		})
	}
}

func (userC *UserController) Update() func(c *gin.Context) {
	return func(c *gin.Context) {
		var userDto users.UserUpdateDto
		id := c.Param("id")
		if !handleBind(userC.t, c, &userDto) {
			return
		}
		user := domain.User{
			UserGeneric: authBaseDomain.NewUserGeneric(
				id,
				userDto.Name,
				userDto.Email,
				"",
				nil,
				userDto.IsAdmin,
				false,
			),
		}
		err := userC.userService.Update(user)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: userC.t.TranslateByLang(c.GetHeader("lang"), "errors.users.update", nil),
				Data:    nil,
			})
		} else {
			c.JSON(http.StatusOK, dto.ResponseDto{
				Status:  http.StatusOK,
				Message: "Ok",
				Data:    nil,
			})
		}
	}
}

func (userC *UserController) ChangeStatus() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		err := userC.userService.ChangeStatus(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: userC.t.TranslateByLang(c.GetHeader("lang"), "errors.users.change-status", nil),
				Data:    nil,
			})
		} else {
			c.JSON(http.StatusOK, dto.ResponseDto{
				Status:  http.StatusOK,
				Message: "Ok",
				Data:    nil,
			})
		}
	}
}
