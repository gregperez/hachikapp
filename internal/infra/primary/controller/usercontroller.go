package controller

import (
	"jwt/base/internal/core"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/infra/config"
	"jwt/base/internal/infra/primary/controller/dto"
	"jwt/base/internal/infra/primary/controller/dto/inputs/users"
	usersOutputs "jwt/base/internal/infra/primary/controller/dto/outputs/users"
	"net/http"

	authBaseCore "github.com/WilsonSayago/authBase/core"
	authBaseDomain "github.com/WilsonSayago/authBase/core/domain"
	"github.com/WilsonSayago/initModules"
	"github.com/gin-gonic/gin"
)

type UserController struct {
	authenticationService authBaseCore.AuthenticationUseCase
	authorizationService  authBaseCore.AuthorizationUseCase[domain.User, *gin.Context]
	userService           core.UserUseCase
	roleService           authBaseCore.RoleUseCase
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
		protected.GET("/full-info",
			userC.authorizationService.PoliciesGuard(userC.FindFullById(), nil,
				"USERS", authBaseDomain.READ))
		protected.PUT("/:id/change-status",
			userC.authorizationService.PoliciesGuard(userC.ChangeStatus(), nil,
				"USERS", authBaseDomain.UPDATE))
		protected.PUT("/:id",
			userC.authorizationService.PoliciesGuard(userC.Update(), nil,
				"USERS", authBaseDomain.UPDATE))
	}
}

// GetAll godoc
// @Summary Get all users
// @Description Retrieves a paginated list of users
// @Tags Users
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(10)
// @Success 200 {object} dto.ResponseDto{data=usersOutputs.UsersPageResponse} "List of users"
// @Failure 400 {object} dto.ResponseDto "Error retrieving users"
// @Security ApiKeyAuth
// @Router /v1/users [get]
func (userC *UserController) GetAll() func(c *gin.Context) {
	return func(c *gin.Context) {
		page, pageSize, offset := handlePagination(c)
		usersDb, total, err := userC.userService.FindAll(pageSize, offset)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: userC.t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
				Data:    nil,
			})
			return
		}

		list := make([]usersOutputs.UserResponse, 0)
		for _, user := range usersDb {
			list = append(list, usersOutputs.UserResponse{
				BaseDto: dto.BaseDto{
					Id:     user.GetId(),
					Active: user.Active,
				},
				Name:    user.GetName(),
				Email:   user.GetEmail(),
				IsAdmin: user.GetIsAdmin()})
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data: usersOutputs.UsersPageResponse{
				Page:       page,
				PageSize:   pageSize,
				TotalItems: total,
				TotalPages: (total + pageSize - 1) / pageSize,
				Data:       list,
			},
		})
	}
}

// Create godoc
// @Summary Create new user
// @Description Creates a new user with the provided details
// @Tags Users
// @Accept json
// @Produce json
// @Param user body users.UseDto true "User details"
// @Success 200 {object} dto.ResponseDto "User created successfully"
// @Failure 400 {object} dto.ResponseDto "Error creating user"
// @Security ApiKeyAuth
// @Router /v1/users [post]
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
				getRolesFromIds(userDto.Roles),
				userDto.IsAdmin,
				false,
			),
		}
		err := userC.userService.Create(user)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: userC.t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
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

func getRolesFromIds(roleIds []string) []authBaseDomain.Role {
	roles := make([]authBaseDomain.Role, 0)
	for _, roleId := range roleIds {
		roles = append(roles, authBaseDomain.Role{Base: authBaseDomain.Base{Id: roleId}})
	}
	return roles
}

// GetById godoc
// @Summary Get user by ID
// @Description Retrieves a user by their ID
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} dto.ResponseDto{data=users.UseDto} "User details"
// @Failure 400 {object} dto.ResponseDto "Error retrieving user"
// @Security ApiKeyAuth
// @Router /v1/users/{id} [get]
func (userC *UserController) GetById() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		user, err := userC.userService.FindById(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: userC.t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
				Data:    nil,
			})
			return
		}

		// userDto := users.UseDto{
		// 	BaseDto: dto.BaseDto{
		// 		Id:     user.GetId(),
		// 		Active: user.Active,
		// 	},
		// 	Name:    user.GetName(),
		// 	Email:   user.GetEmail(),
		// 	IsAdmin: user.GetIsAdmin(),
		// }
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    usersOutputs.MapDomainToUserResponse(user),
		})
	}
}

// FindFullById godoc
// @Summary Get full user info
// @Description Retrieves complete user information including roles and permissions
// @Tags Users
// @Accept json
// @Produce json
// @Success 200 {object} dto.ResponseDto{data=usersOutputs.UserResponse} "Full user details"
// @Failure 400 {object} dto.ResponseDto "Error retrieving user info"
// @Security ApiKeyAuth
// @Router /v1/users/full-info [get]
func (userC *UserController) FindFullById() func(c *gin.Context) {
	return func(c *gin.Context) {
		userToken, _ := c.Get("user")
		user, err := userC.userService.FindFullById(userToken.(domain.User).GetId())
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: userC.t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
				Data:    nil,
			})
			return
		}
		response := usersOutputs.UserResponse{}
		response.MapDomainToResponse(user)

		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    response,
		})
	}
}

// Update godoc
// @Summary Update user
// @Description Updates an existing user with the provided details
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param user body users.UserUpdateDto true "User update details"
// @Success 200 {object} dto.ResponseDto "User updated successfully"
// @Failure 400 {object} dto.ResponseDto "Error updating user"
// @Security ApiKeyAuth
// @Router /v1/users/{id} [put]
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
				getRolesFromIds(userDto.Roles),
				userDto.IsAdmin,
				false,
			),
		}

		err := userC.userService.Update(user)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: userC.t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
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

// ChangeStatus godoc
// @Summary Change user status
// @Description Changes the active status of a user
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} dto.ResponseDto "Status changed successfully"
// @Failure 400 {object} dto.ResponseDto "Error changing status"
// @Security ApiKeyAuth
// @Router /v1/users/change-status/{id} [put]
func (userC *UserController) ChangeStatus() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		err := userC.userService.ChangeStatus(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: userC.t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
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
