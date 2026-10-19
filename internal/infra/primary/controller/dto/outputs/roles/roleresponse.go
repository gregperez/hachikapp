package roles

import (
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto"

	"github.com/WilsonSayago/authBase/v4/core/domain"
)

// RoutePathEnumDto extiende domain.RoutePathEnum para Swagger
// @Description Enumeración de rutas en el sistema
type RoutePathEnumDto string

const (
	USERS        RoutePathEnumDto = RoutePathEnumDto(domain.USERS)
	ROLES        RoutePathEnumDto = RoutePathEnumDto(domain.ROLES)
	AUDIT_EVENTS RoutePathEnumDto = "AUDIT_EVENTS"
	SETTINGS     RoutePathEnumDto = "SETTINGS"
	TAGS         RoutePathEnumDto = "TAGS"
)

// RoleResponse represents a role response.
// @swagger:model
// @Description RoleResponse represents a role response.
type RoleResponse struct {
	dto.BaseDto
	Name        string               `json:"name,omitempty"`
	Routes      []RoutePathEnumDto   `json:"routes,omitempty"`
	Permissions []PermissionResponse `json:"permissions,omitempty"`
}

// convertRoutesToDto
func ConvertRoutesToDto(routes []domain.RoutePathEnum) []RoutePathEnumDto {
	var routesDto []RoutePathEnumDto
	for _, route := range routes {
		routesDto = append(routesDto, RoutePathEnumDto(route))
	}
	return routesDto
}

func MapDomainToResponse(role domain.Role) RoleResponse {
	var response RoleResponse
	response.BaseDto.DomainToEntity(role.Base)
	response.Name = role.GetName()
	response.Routes = ConvertRoutesToDto(role.GetRoutes())
	response.Permissions = make([]PermissionResponse, 0, len(role.GetPermissions()))
	for _, permission := range role.GetPermissions() {
		var item PermissionResponse
		item.MapToEntity(permission)
		response.Permissions = append(response.Permissions, item)
	}
	return response
}
