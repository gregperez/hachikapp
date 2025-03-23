package roles

import (
	"jwt/base/internal/infra/primary/controller/dto"

	"github.com/WilsonSayago/authBase/core/domain"
)

// RoutePathEnumDto extiende domain.RoutePathEnum para Swagger
// @Description Enumeración de rutas en el sistema
type RoutePathEnumDto string

const (
	USERS RoutePathEnumDto = RoutePathEnumDto(domain.USERS) // 🔥 Hereda el valor del paquete externo
	ROLES RoutePathEnumDto = RoutePathEnumDto(domain.ROLES)
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
	return response
}
