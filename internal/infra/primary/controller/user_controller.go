package controller

import (
	"net/http"

	"github.com/WilsonSayago/base-golang/internal/core"
	"github.com/WilsonSayago/base-golang/internal/core/authz"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto"
	users "github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto/inputs/users"
	userout "github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto/outputs/users"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/ginadapter"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/httpapi"

	authBaseCore "github.com/WilsonSayago/authBase/v3/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v3/core/domain"
	initModules "github.com/WilsonSayago/initModules/v2"
	"github.com/gin-gonic/gin"
)

type UserController struct {
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	userService          core.UserUseCase
	r                    *gin.Engine
	t                    config.ITranslate
}

func GetUserControllerInstance(
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	userService core.UserUseCase,
	r *gin.Engine,
	t config.ITranslate,
) *UserController {
	return initModules.Once(func() *UserController {
		return &UserController{
			authorizationService: authorizationService,
			userService:          userService,
			r:                    r,
			t:                    t,
		}
	})
}

func (userC *UserController) RunController() {
	protected := userC.r.Group("/v1/users")
	protected.Use(ginadapter.Middleware(userC.authorizationService.AuthorizeJWT()))
	{
		protected.GET("/", ginadapter.Middleware(userC.authorizationService.PoliciesGuard(
			ginadapter.Handler(userC.GetAll()), nil, authz.EntityUsers, authz.Read)))
		protected.POST("/", ginadapter.Middleware(userC.authorizationService.PoliciesGuard(
			ginadapter.Handler(userC.Create()), nil, authz.EntityUsers, authz.Create)))

		// Own-profile routes before /:id.
		protected.GET("/me", userC.FindFullById())
		protected.PUT("/me", userC.UpdateOwnProfile())
		protected.PUT("/me/password", userC.ChangePassword())
		protected.GET("/full-info", userC.FindFullById())

		protected.GET("/:id", ginadapter.Middleware(userC.authorizationService.PoliciesGuard(
			ginadapter.Handler(userC.GetById()), nil, authz.EntityUsers, authz.Read)))
		protected.PUT("/:id", ginadapter.Middleware(userC.authorizationService.PoliciesGuard(
			ginadapter.Handler(userC.AdminUpdate()), nil, authz.EntityUsers, authz.Update)))
		protected.PUT("/:id/change-status", ginadapter.Middleware(userC.authorizationService.PoliciesGuard(
			ginadapter.Handler(userC.ChangeStatus()), nil, authz.EntityUsers, authz.Update)))
		protected.PUT("/:id/roles", ginadapter.Middleware(userC.authorizationService.PoliciesGuard(
			ginadapter.Handler(userC.AssignRoles()), nil, authz.EntityUsersPrivileges, authz.Update)))
		protected.PUT("/:id/admin", ginadapter.Middleware(userC.authorizationService.PoliciesGuard(
			ginadapter.Handler(userC.SetAdminFlag()), nil, authz.EntityUsersPrivileges, authz.Update)))
	}
}

// GetAll godoc
// @Summary List users
// @Description Retrieves a paginated list of users (stable order by repository)
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param page query int false "Page number (>=1)" default(1)
// @Param pageSize query int false "Page size (1-100)" default(10) maximum(100)
// @Success 200 {object} dto.ResponseDto{data=userout.UsersPageResponse}
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Router /v1/users [get]
func (userC *UserController) GetAll() func(c *gin.Context) {
	return func(c *gin.Context) {
		page, pageSize, offset, ok := handlePagination(c)
		if !ok {
			return
		}
		usersDb, total, err := userC.userService.FindAll(c.Request.Context(), pageSize, offset)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}

		list := make([]userout.UserResponse, 0, len(usersDb))
		for _, user := range usersDb {
			list = append(list, userout.UserResponse{
				BaseDto: dto.BaseDto{
					Id:     user.GetId(),
					Active: user.Active,
				},
				Name:    user.GetName(),
				Email:   user.GetEmail(),
				IsAdmin: user.GetIsAdmin(),
			})
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data: userout.UsersPageResponse{
				Page:       page,
				PageSize:   pageSize,
				TotalItems: total,
				TotalPages: httpapi.TotalPages(total, pageSize),
				Data:       list,
			},
		})
	}
}

// Create godoc
// @Summary Create user
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body users.CreateUserRequest true "User"
// @Success 201 {object} dto.ResponseDto
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Router /v1/users [post]
func (userC *UserController) Create() func(c *gin.Context) {
	return func(c *gin.Context) {
		var req users.CreateUserRequest
		if !handleBind(userC.t, c, &req) {
			return
		}
		user := domain.WithPlainPassword(domain.User{
			UserGeneric: authBaseDomain.NewUserGeneric(
				"",
				req.Name,
				req.Email,
				nil,
				false,
				true,
			),
		}, req.Password)
		if err := userC.userService.Create(c.Request.Context(), user); err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.JSON(http.StatusCreated, dto.ResponseDto{
			Status:  http.StatusCreated,
			Message: "Ok",
			Data:    nil,
		})
	}
}

// GetById godoc
// @Summary Get user by ID
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} dto.ResponseDto{data=userout.UserResponse}
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Router /v1/users/{id} [get]
func (userC *UserController) GetById() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		user, err := userC.userService.FindById(c.Request.Context(), id)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    userout.MapDomainToUserResponse(user),
		})
	}
}

// FindFullById godoc
// @Summary Get current user profile
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.ResponseDto{data=userout.UserResponse}
// @Failure 401 {object} httpapi.Problem
// @Router /v1/users/me [get]
func (userC *UserController) FindFullById() func(c *gin.Context) {
	return func(c *gin.Context) {
		userToken, ok := requireIdentity(userC.authorizationService, c)
		if !ok {
			return
		}
		user, err := userC.userService.FindFullById(c.Request.Context(), userToken.GetId())
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		response := userout.UserResponse{}
		response.MapDomainToResponse(user)
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    response,
		})
	}
}

// UpdateOwnProfile godoc
// @Summary Update own profile
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body users.UpdateOwnProfileRequest true "Profile"
// @Success 200 {object} dto.ResponseDto
// @Failure 401 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Router /v1/users/me [put]
func (userC *UserController) UpdateOwnProfile() func(c *gin.Context) {
	return func(c *gin.Context) {
		userToken, ok := requireIdentity(userC.authorizationService, c)
		if !ok {
			return
		}
		var req users.UpdateOwnProfileRequest
		if !handleBind(userC.t, c, &req) {
			return
		}
		if err := userC.userService.UpdateOwnProfile(c.Request.Context(), userToken.GetId(), userToken.GetId(), req.Name); err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: nil})
	}
}

// ChangePassword godoc
// @Summary Change own password
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body users.ChangePasswordRequest true "Passwords"
// @Success 200 {object} dto.ResponseDto
// @Failure 401 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Router /v1/users/me/password [put]
func (userC *UserController) ChangePassword() func(c *gin.Context) {
	return func(c *gin.Context) {
		userToken, ok := requireIdentity(userC.authorizationService, c)
		if !ok {
			return
		}
		var req users.ChangePasswordRequest
		if !handleBind(userC.t, c, &req) {
			return
		}
		if err := userC.userService.ChangePassword(c.Request.Context(), userToken.GetId(), req.CurrentPassword, req.NewPassword); err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: nil})
	}
}

// AdminUpdate godoc
// @Summary Admin update user
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body users.AdminUpdateUserRequest true "User"
// @Success 200 {object} dto.ResponseDto
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Router /v1/users/{id} [put]
func (userC *UserController) AdminUpdate() func(c *gin.Context) {
	return func(c *gin.Context) {
		var req users.AdminUpdateUserRequest
		id := c.Param("id")
		if !handleBind(userC.t, c, &req) {
			return
		}
		if err := userC.userService.AdminUpdateUser(c.Request.Context(), id, req.Name, req.Email); err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: nil})
	}
}

// AssignRoles godoc
// @Summary Assign roles to user
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body users.AssignUserRolesRequest true "Roles"
// @Success 200 {object} dto.ResponseDto
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Router /v1/users/{id}/roles [put]
func (userC *UserController) AssignRoles() func(c *gin.Context) {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(userC.authorizationService, c)
		if !ok {
			return
		}
		var req users.AssignUserRolesRequest
		if !handleBind(userC.t, c, &req) {
			return
		}
		if err := userC.userService.AssignRoles(c.Request.Context(), actor.GetId(), c.Param("id"), req.RoleIDs); err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: nil})
	}
}

// SetAdminFlag godoc
// @Summary Set admin flag
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body users.SetAdminFlagRequest true "Flag"
// @Success 200 {object} dto.ResponseDto
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Failure 409 {object} httpapi.Problem
// @Router /v1/users/{id}/admin [put]
func (userC *UserController) SetAdminFlag() func(c *gin.Context) {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(userC.authorizationService, c)
		if !ok {
			return
		}
		var req users.SetAdminFlagRequest
		if !handleBind(userC.t, c, &req) {
			return
		}
		if err := userC.userService.SetAdminFlag(c.Request.Context(), actor.GetId(), c.Param("id"), *req.IsAdmin); err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: nil})
	}
}

// ChangeStatus godoc
// @Summary Change user active status
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body users.SetActiveRequest true "Active flag"
// @Success 200 {object} dto.ResponseDto
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Failure 409 {object} httpapi.Problem
// @Router /v1/users/{id}/change-status [put]
func (userC *UserController) ChangeStatus() func(c *gin.Context) {
	return func(c *gin.Context) {
		actor, ok := requireIdentity(userC.authorizationService, c)
		if !ok {
			return
		}
		var req users.SetActiveRequest
		if !handleBind(userC.t, c, &req) {
			return
		}
		if err := userC.userService.SetActive(c.Request.Context(), actor.GetId(), c.Param("id"), *req.Active); err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: nil})
	}
}

func requireIdentity(authz authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context], c *gin.Context) (domain.User, bool) {
	userToken, ok := authz.GetUserToken(ginadapter.Wrap(c))
	if !ok {
		httpapi.WriteProblem(c, httpapi.Unauthorized("Authentication required", httpapi.RequestID(c)))
		return domain.User{}, false
	}
	return userToken, true
}
