package instance

import (
	middlewareCore "github.com/WilsonSayago/middleware/core"
	middlewareService "github.com/WilsonSayago/middleware/core/services"
	"github.com/WilsonSayago/middleware/infra/config/properties"
	"jwt/base/internal/core/domain"
)

func MiddlewareUseCaseInstance() middlewareCore.MiddlewareUseCase {
	return middlewareService.GetMiddlewareInstance[domain.User](GetUserInstance(), properties.GetMiddlewareProp())
}
