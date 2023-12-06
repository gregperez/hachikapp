package instance

import (
	"jwt/base/internal/core"
	"jwt/base/internal/core/services"
)

func MiddlewareUseCaseInstance() *core.MiddlewareUseCase {
	return services.GetMiddlewareInstance(GetUserInstance())
}
