package controller

import (
	"fmt"
	"net/http"

	"github.com/gregperez/hachikapp-backend/internal/core/authz"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/config"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/inputs/users"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/outputs/roles"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/ginadapter"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gin-gonic/gin"
)

type RoleController struct {
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	roleService          authBaseCore.RoleUseCase
	r                    *gin.Engine
	t                    config.ITranslate
}

func NewRoleController(
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	roleService authBaseCore.RoleUseCase,
	r *gin.Engine,
	t config.ITranslate,
) *RoleController {
	return &RoleController{
		authorizationService: authorizationService,
		roleService:          roleService,
		r:                    r,
		t:                    t,
	}
}

// GetRoleControllerInstance is an alias for NewRoleController.
//
// Deprecated: use NewRoleController.
func GetRoleControllerInstance(
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	roleService authBaseCore.RoleUseCase,
	r *gin.Engine,
	t config.ITranslate,
) *RoleController {
	return NewRoleController(authorizationService, roleService, r, t)
}

func (roleC *RoleController) RunController() {
	protected := roleC.r.Group("/v1/roles")
	protected.Use(ginadapter.Middleware(roleC.authorizationService.AuthorizeJWT()))
	{
		protected.GET("/", ginadapter.Middleware(roleC.authorizationService.PoliciesGuard(
			ginadapter.Handler(roleC.GetAll()), nil, authz.EntityRoles, authz.Read)))
		protected.GET("/:id", ginadapter.Middleware(roleC.authorizationService.PoliciesGuard(
			ginadapter.Handler(roleC.GetById()), nil, authz.EntityRoles, authz.Read)))
		protected.POST("/", ginadapter.Middleware(roleC.authorizationService.PoliciesGuard(
			ginadapter.Handler(roleC.Save()), nil, authz.EntityRoles, authz.Create)))
		protected.PUT("/:id/change-status", ginadapter.Middleware(roleC.authorizationService.PoliciesGuard(
			ginadapter.Handler(roleC.ChangeStatus()), nil, authz.EntityRoles, authz.Update)))
		protected.PUT("/:id", ginadapter.Middleware(roleC.authorizationService.PoliciesGuard(
			ginadapter.Handler(roleC.Update()), nil, authz.EntityRoles, authz.Update)))
	}
}

// GetAll godoc
// @Summary Get all roles
// @Description Retrieves roles ordered by createdAt and id descending
// @Tags Roles
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param limit query int false "Page size (1-100)" default(10) maximum(100)
// @Param cursor query string false "Opaque cursor returned by the previous page"
// @Success 200 {object} dto.ResponseDto{data=roles.RolesPageResponse} "List of roles"
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Router /v1/roles [get]
func (roleC *RoleController) GetAll() func(c *gin.Context) {
	return func(c *gin.Context) {
		request, ok := handlePagination(c)
		if !ok {
			return
		}
		page, err := roleC.roleService.GetRoles(c.Request.Context(), request)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}

		list := make([]roles.RoleResponse, 0, len(page.Items))
		for _, role := range page.Items {
			list = append(list, roles.MapDomainToResponse(role))
		}

		nextCursor := ""
		if page.HasNext && len(page.Items) > 0 {
			last := page.Items[len(page.Items)-1]
			nextCursor, err = httpapi.EncodeCursor(authBaseDomain.CursorKey{
				CreatedAt: last.CreatedAt,
				ID:        last.GetId(),
			})
			if err != nil {
				httpapi.WriteProblem(c, httpapi.MapError(
					fmt.Errorf("encode roles cursor: %v", err),
					httpapi.RequestID(c),
				))
				return
			}
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data: roles.RolesPageResponse{
				Limit:      request.Limit,
				HasNext:    page.HasNext,
				NextCursor: nextCursor,
				Data:       list,
			},
		})
	}
}

// GetById godoc
// @Summary Get role by ID
// @Description Retrieves a role by its ID
// @Tags Roles
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} dto.ResponseDto{data=roles.RoleResponse} "Role details"
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Failure 404 {object} httpapi.Problem
// @Router /v1/roles/{id} [get]
func (roleC *RoleController) GetById() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		role, err := roleC.roleService.GetRoleById(c.Request.Context(), id)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}

		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    roles.MapDomainToResponse(role),
		})
	}
}

// Save godoc
// @Summary Create new role
// @Description Creates a new role with the provided details
// @Tags Roles
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param role body dto.RoleDto true "Role details"
// @Success 201 {object} dto.ResponseDto "Role created successfully"
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Router /v1/roles [post]
func (roleC *RoleController) Save() func(c *gin.Context) {
	return func(c *gin.Context) {
		roleDto := dto.RoleDto{}
		if !handleBind(roleC.t, c, &roleDto) {
			return
		}

		role := authBaseDomain.Role{
			Name:        roleDto.Name,
			Routes:      roleDto.Routes,
			Permissions: convertDtoToPermissions(roleDto.Permissions),
		}

		_, err := roleC.roleService.CreateRole(c.Request.Context(), role)
		if err != nil {
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

// Update godoc
// @Summary Update role
// @Description Updates an existing role with the provided details
// @Tags Roles
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param role body dto.RoleDto true "Role details"
// @Success 200 {object} dto.ResponseDto "Role updated successfully"
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Failure 422 {object} httpapi.Problem
// @Router /v1/roles/{id} [put]
func (roleC *RoleController) Update() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		roleDto := dto.RoleDto{}
		if !handleBind(roleC.t, c, &roleDto) {
			return
		}

		role := authBaseDomain.Role{
			Base:        authBaseDomain.Base{Id: id},
			Name:        roleDto.Name,
			Routes:      roleDto.Routes,
			Permissions: convertDtoToPermissions(roleDto.Permissions),
		}

		err := roleC.roleService.UpdateRole(c.Request.Context(), role)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}

		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    nil,
		})
	}
}

// ChangeStatus godoc
// @Summary Change role status
// @Description Changes the active status of a role
// @Tags Roles
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param body body users.SetActiveRequest true "Active flag"
// @Success 200 {object} dto.ResponseDto "Status changed successfully"
// @Failure 400 {object} httpapi.Problem
// @Failure 401 {object} httpapi.Problem
// @Failure 403 {object} httpapi.Problem
// @Router /v1/roles/{id}/change-status [put]
func (roleC *RoleController) ChangeStatus() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req users.SetActiveRequest
		if !handleBind(roleC.t, c, &req) {
			return
		}
		err := roleC.roleService.SetActive(c.Request.Context(), id, *req.Active)
		if err != nil {
			httpapi.WriteProblem(c, httpapi.MapError(err, httpapi.RequestID(c)))
			return
		}

		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    nil,
		})
	}
}

func convertDtoToPermissions(permissionDtos []dto.PermissionDto) []authBaseDomain.Permission {
	permissions := make([]authBaseDomain.Permission, len(permissionDtos))
	for i, permissionDto := range permissionDtos {
		permissions[i] = authBaseDomain.Permission{
			Entity: permissionDto.Entity,
			Create: permissionDto.Create,
			Read:   permissionDto.Read,
			Update: permissionDto.Update,
			Delete: permissionDto.Delete,
		}
	}
	return permissions
}
