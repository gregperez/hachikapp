package controller

import (
	"jwt/base/internal/core/domain"
	"jwt/base/internal/infra/primary/controller/dto"
	"net/http"

	authBaseCore "github.com/WilsonSayago/authBase/core"
	authBaseDomain "github.com/WilsonSayago/authBase/core/domain"
	"github.com/WilsonSayago/initModules"
	"github.com/gin-gonic/gin"
)

type RoleController struct {
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *gin.Context]
	roleService          authBaseCore.RoleUseCase
	r                    *gin.Engine
}

func GetRoleControllerInstance(
	authorizationService authBaseCore.AuthorizationUseCase[domain.User, *gin.Context],
	roleService authBaseCore.RoleUseCase,
	r *gin.Engine) *RoleController {
	instance := initModules.GetInstance("RoleController", func() interface{} {
		return &RoleController{
			authorizationService: authorizationService,
			roleService:          roleService,
			r:                    r,
		}
	})
	return instance.(*RoleController)
}

func (roleC *RoleController) Start() {
	roleC.RunController()
}

func (roleC *RoleController) RunController() {
	protected := (*roleC.r).Group("/roles")
	protected.Use(roleC.authorizationService.AuthorizeJWT())
	{
		protected.GET("/",
			roleC.authorizationService.PoliciesGuard(roleC.GetAll(), nil,
				"USERS", authBaseDomain.READ))
		protected.GET("/:id",
			roleC.authorizationService.PoliciesGuard(roleC.GetById(), nil,
				"USERS", authBaseDomain.READ))
		// endpoint save
		protected.POST("/", roleC.authorizationService.PoliciesGuard(roleC.Save(), nil,
			"USERS", authBaseDomain.CREATE))
	}
}

func (roleC *RoleController) GetAll() func(c *gin.Context) {
	return func(c *gin.Context) {
		roles, err := roleC.roleService.GetRoles()
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: "Error getting roles",
				Data:    nil,
			})
			return
		}

		list := make([]dto.RoleDto, 0)
		for _, role := range roles {
			var r dto.RoleDto = dto.RoleDto{
				Id:          role.GetId(),
				Name:        role.GetName(),
				Routes:      role.GetRoutes(),
				Permissions: convertPermissionsToDto(role.GetPermissions()),
			}
			list = append(list, r)
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Get roles",
			Data:    list,
		})
	}
}

func (roleC *RoleController) GetById() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		role, err := roleC.roleService.GetRoleById(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: "Error getting role",
				Data:    nil,
			})
		}

		roleDto := dto.RoleDto{
			Id:          role.GetId(),
			Name:        role.GetName(),
			Routes:      role.GetRoutes(),
			Permissions: convertPermissionsToDto(role.GetPermissions()),
		}
		c.JSON(http.StatusOK, dto.ResponseDto{
			Status:  http.StatusOK,
			Message: "Get role",
			Data:    roleDto,
		})
	}
}

func (roleC *RoleController) Save() func(c *gin.Context) {
	return func(c *gin.Context) {
		roleDto := dto.RoleDto{}
		err := c.ShouldBindJSON(&roleDto)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: "Error binding role",
				Data:    nil,
			})
			return
		}

		role := authBaseDomain.Role{
			Name:        roleDto.Name,
			Routes:      roleDto.Routes,
			Permissions: convertDtoToPermissions(roleDto.Permissions),
		}

		_, err = roleC.roleService.CreateRole(role)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: "Error creating role",
				Data:    nil,
			})
			return
		}

		c.JSON(http.StatusCreated, dto.ResponseDto{
			Status:  http.StatusCreated,
			Message: "Role created",
			Data:    nil,
		})
	}
}

func convertPermissionsToDto(permissions []authBaseDomain.Permission) []dto.PermissionDto {
	permissionDtos := make([]dto.PermissionDto, len(permissions))
	for i, permission := range permissions {
		permissionDtos[i] = dto.PermissionDto{
			Entity: permission.Entity,
			Create: permission.Create,
			Read:   permission.Read,
			Update: permission.Update,
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
		}
	}
	return permissions
}
