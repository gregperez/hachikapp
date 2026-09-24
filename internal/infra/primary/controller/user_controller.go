package controller

import (
	"errors"
	"net/http"

	"github.com/WilsonSayago/base-golang/internal/core"
	"github.com/WilsonSayago/base-golang/internal/core/authz"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto/inputs/users"
	usersOutputs "github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto/outputs/users"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/ginadapter"

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

func (userC *UserController) GetAll() func(c *gin.Context) {
	return func(c *gin.Context) {
		page, pageSize, offset := handlePagination(c)
		usersDb, total, err := userC.userService.FindAll(pageSize, offset)
		if err != nil {
			writeTranslatedError(userC.t, c, http.StatusBadRequest, err)
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
		if err := userC.userService.Create(user); err != nil {
			writeUseCaseError(userC.t, c, err)
			return
		}
		c.JSON(http.StatusCreated, dto.ResponseDto{
			Status:  http.StatusCreated,
			Message: "Ok",
			Data:    nil,
		})
	}
}

func (userC *UserController) GetById() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		user, err := userC.userService.FindById(id)
		if err != nil {
			writeTranslatedError(userC.t, c, http.StatusBadRequest, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    usersOutputs.MapDomainToUserResponse(user),
		})
	}
}

func (userC *UserController) FindFullById() func(c *gin.Context) {
	return func(c *gin.Context) {
		userToken, ok := userC.authorizationService.GetUserToken(ginadapter.Wrap(c))
		if !ok {
			c.JSON(http.StatusUnauthorized, dto.ResponseDto{
				Status:  http.StatusUnauthorized,
				Message: "unauthorized",
				Data:    nil,
			})
			return
		}
		user, err := userC.userService.FindFullById(userToken.GetId())
		if err != nil {
			writeTranslatedError(userC.t, c, http.StatusBadRequest, err)
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

func (userC *UserController) UpdateOwnProfile() func(c *gin.Context) {
	return func(c *gin.Context) {
		userToken, ok := userC.authorizationService.GetUserToken(ginadapter.Wrap(c))
		if !ok {
			c.JSON(http.StatusUnauthorized, dto.ResponseDto{
				Status:  http.StatusUnauthorized,
				Message: "unauthorized",
				Data:    nil,
			})
			return
		}
		var req users.UpdateOwnProfileRequest
		if !handleBind(userC.t, c, &req) {
			return
		}
		if err := userC.userService.UpdateOwnProfile(userToken.GetId(), userToken.GetId(), req.Name); err != nil {
			writeUseCaseError(userC.t, c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: nil})
	}
}

func (userC *UserController) ChangePassword() func(c *gin.Context) {
	return func(c *gin.Context) {
		userToken, ok := userC.authorizationService.GetUserToken(ginadapter.Wrap(c))
		if !ok {
			c.JSON(http.StatusUnauthorized, dto.ResponseDto{
				Status:  http.StatusUnauthorized,
				Message: "unauthorized",
				Data:    nil,
			})
			return
		}
		var req users.ChangePasswordRequest
		if !handleBind(userC.t, c, &req) {
			return
		}
		if err := userC.userService.ChangePassword(c.Request.Context(), userToken.GetId(), req.CurrentPassword, req.NewPassword); err != nil {
			writeUseCaseError(userC.t, c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: nil})
	}
}

func (userC *UserController) AdminUpdate() func(c *gin.Context) {
	return func(c *gin.Context) {
		var req users.AdminUpdateUserRequest
		id := c.Param("id")
		if !handleBind(userC.t, c, &req) {
			return
		}
		if err := userC.userService.AdminUpdateUser(id, req.Name, req.Email); err != nil {
			writeUseCaseError(userC.t, c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: nil})
	}
}

func (userC *UserController) AssignRoles() func(c *gin.Context) {
	return func(c *gin.Context) {
		actor, ok := userC.authorizationService.GetUserToken(ginadapter.Wrap(c))
		if !ok {
			c.JSON(http.StatusUnauthorized, dto.ResponseDto{
				Status:  http.StatusUnauthorized,
				Message: "unauthorized",
				Data:    nil,
			})
			return
		}
		var req users.AssignUserRolesRequest
		if !handleBind(userC.t, c, &req) {
			return
		}
		if err := userC.userService.AssignRoles(actor.GetId(), c.Param("id"), req.RoleIDs); err != nil {
			writeUseCaseError(userC.t, c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: nil})
	}
}

func (userC *UserController) SetAdminFlag() func(c *gin.Context) {
	return func(c *gin.Context) {
		actor, ok := userC.authorizationService.GetUserToken(ginadapter.Wrap(c))
		if !ok {
			c.JSON(http.StatusUnauthorized, dto.ResponseDto{
				Status:  http.StatusUnauthorized,
				Message: "unauthorized",
				Data:    nil,
			})
			return
		}
		var req users.SetAdminFlagRequest
		if !handleBind(userC.t, c, &req) {
			return
		}
		if err := userC.userService.SetAdminFlag(actor.GetId(), c.Param("id"), req.IsAdmin); err != nil {
			writeUseCaseError(userC.t, c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: nil})
	}
}

func (userC *UserController) ChangeStatus() func(c *gin.Context) {
	return func(c *gin.Context) {
		actor, ok := userC.authorizationService.GetUserToken(ginadapter.Wrap(c))
		if !ok {
			c.JSON(http.StatusUnauthorized, dto.ResponseDto{
				Status:  http.StatusUnauthorized,
				Message: "unauthorized",
				Data:    nil,
			})
			return
		}
		if err := userC.userService.ChangeStatus(actor.GetId(), c.Param("id")); err != nil {
			writeUseCaseError(userC.t, c, err)
			return
		}
		c.JSON(http.StatusOK, dto.ResponseDto{Status: http.StatusOK, Message: "Ok", Data: nil})
	}
}

func writeUseCaseError(t config.ITranslate, c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrForbidden):
		c.JSON(http.StatusForbidden, dto.ResponseDto{
			Status:  http.StatusForbidden,
			Message: "forbidden",
			Data:    nil,
		})
	case errors.Is(err, domain.ErrLastAdmin):
		c.JSON(http.StatusConflict, dto.ResponseDto{
			Status:  http.StatusConflict,
			Message: t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
			Data:    nil,
		})
	case errors.Is(err, domain.ErrWeakPassword):
		c.JSON(http.StatusBadRequest, dto.ResponseDto{
			Status:  http.StatusBadRequest,
			Message: "weak password",
			Data:    nil,
		})
	default:
		writeTranslatedError(t, c, http.StatusBadRequest, err)
	}
}

func writeTranslatedError(t config.ITranslate, c *gin.Context, status int, err error) {
	c.JSON(status, dto.ResponseDto{
		Status:  status,
		Message: t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
		Data:    nil,
	})
}
