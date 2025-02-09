package roles

import (
	"github.com/WilsonSayago/authBase/core/domain"
	"jwt/base/internal/infra/primary/controller/dto"
)

type RoleResponse struct {
	dto.BaseDto
	Name        string                 `json:"name,omitempty"`
	Routes      []domain.RoutePathEnum `json:"routes,omitempty"`
	Permissions []PermissionResponse   `json:"permissions,omitempty"`
}
