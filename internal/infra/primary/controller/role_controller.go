package controller

import (
	"net/http"

	"github.com/WilsonSayago/base-golang/internal/core/authz"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto/outputs/roles"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/ginadapter"

	authBaseCore "github.com/WilsonSayago/authBase/v3/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v3/core/domain"
	initModules "github.com/WilsonSayago/initModules/v2"
	"github.com/gin-gonic/gin"
)

type RoleController struct {
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	roleService          authBaseCore.RoleUseCase
	r                    *gin.Engine
	t                    config.ITranslate
}

func GetRoleControllerInstance(
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context],
	roleService authBaseCore.RoleUseCase,
	r *gin.Engine,
	t config.ITranslate,
) *RoleController {
	return initModules.Once(func() *RoleController {
		return &RoleController{
			authorizationService: authorizationService,
			roleService:          roleService,
			r:                    r,
			t:                    t,
		}
	})
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
// @Description Retrieves a paginated list of roles
// @Tags Roles
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(10)
// @Success 200 {object} dto.ResponseDto{data=roles.RolesPageResponse} "List of roles"
// @Failure 400 {object} dto.ResponseDto "Error retrieving roles"
// @Router /v1/roles [get]
func (roleC *RoleController) GetAll() func(c *gin.Context) {
	return func(c *gin.Context) {
		page, pageSize, offset := handlePagination(c)
		rolesDb, total, err := roleC.roleService.GetRoles(c.Request.Context(), pageSize, offset)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: roleC.t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
				Data:    nil,
			})
			return
		}

		list := make([]roles.RoleResponse, 0)
		for _, role := range rolesDb {
			var r = roles.RoleResponse{
				BaseDto: dto.BaseDto{
					Id:     role.GetId(),
					Active: role.GetActive(),
				},
				Name:        role.GetName(),
				Routes:      roles.ConvertRoutesToDto(role.GetRoutes()),
				Permissions: convertPermissionsToDto(role.GetPermissions()),
			}
			list = append(list, r)
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data: roles.RolesPageResponse{
				Page:       page,
				PageSize:   pageSize,
				TotalItems: total,
				TotalPages: (total + pageSize - 1) / pageSize,
				Data:       list,
			},
		})
	}
}

// GetById godoc
// @Summary Get role by ID
// @Description Retrieves a role by its ID
// @Tags Roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} dto.ResponseDto{data=roles.RoleResponse} "Role details"
// @Failure 400 {object} dto.ResponseDto "Error retrieving role"
// @Router /v1/roles/{id} [get]
func (roleC *RoleController) GetById() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		role, err := roleC.roleService.GetRoleById(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: roleC.t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
				Data:    nil,
			})
			return
		}

		roleDto := roles.RoleResponse{
			BaseDto: dto.BaseDto{
				Id:     role.GetId(),
				Active: role.GetActive(),
			},
			Name:        role.GetName(),
			Routes:      roles.ConvertRoutesToDto(role.GetRoutes()),
			Permissions: convertPermissionsToDto(role.GetPermissions()),
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    roleDto,
		})
	}
}

// Save godoc
// @Summary Create new role
// @Description Creates a new role with the provided details
// @Tags Roles
// @Accept json
// @Produce json
// @Param role body dto.RoleDto true "Role details"
// @Success 201 {object} dto.ResponseDto "Role created successfully"
// @Failure 400 {object} dto.ResponseDto "Error creating role"
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
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: roleC.t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
				Data:    nil,
			})
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
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param role body dto.RoleDto true "Role details"
// @Success 200 {object} dto.ResponseDto "Role updated successfully"
// @Failure 400 {object} dto.ResponseDto "Error updating role"
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
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: roleC.t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
				Data:    nil,
			})
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
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} dto.ResponseDto "Status changed successfully"
// @Failure 400 {object} dto.ResponseDto "Error changing status"
// @Router /v1/roles/change-status/{id} [put]
func (roleC *RoleController) ChangeStatus() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Active bool `json:"active"`
		}
		if !handleBind(roleC.t, c, &req) {
			return
		}
		err := roleC.roleService.SetActive(c.Request.Context(), id, req.Active)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: roleC.t.TranslateByLang(c.GetHeader("lang"), err.Error(), nil),
				Data:    nil,
			})
			return
		}

		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Ok",
			Data:    nil,
		})
	}
}

func convertPermissionsToDto(permissions []authBaseDomain.Permission) []roles.PermissionResponse {
	permissionDtos := make([]roles.PermissionResponse, len(permissions))
	for i, permission := range permissions {
		permissionDtos[i] = roles.PermissionResponse{
			Entity: permission.Entity,
			Create: permission.Create,
			Read:   permission.Read,
			Update: permission.Update,
			Delete: permission.Delete,
		}
	}
	return permissionDtos
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
