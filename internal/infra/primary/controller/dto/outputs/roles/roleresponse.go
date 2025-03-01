package roles

import (
	"github.com/WilsonSayago/authBase/core/domain"
	"jwt/base/internal/infra/primary/controller/dto"
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
