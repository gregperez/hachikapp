package instance

import (
	"github.com/WilsonSayago/initModules"
	middlewareCore "github.com/WilsonSayago/middleware/core"
	middlewareService "github.com/WilsonSayago/middleware/core/services"
	"github.com/WilsonSayago/middleware/infra/config/properties"
	"github.com/gin-gonic/gin"
	"jwt/base/internal/core"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/core/services"
)

func MiddlewareUseCaseInstance() middlewareCore.MiddlewareUseCase {
	return middlewareService.GetMiddlewareInstance[domain.User](GetUserRepositoryInstance(),
		initModules.NewInstance[properties.MiddlewareProp]().GetInstance(properties.NewMiddlewareProp))
}

func AuthenticationUseCaseInstance() middlewareCore.AuthenticationUseCase {
	return middlewareService.GetAuthenticationInstance[domain.User](
		GetUserRepositoryInstance(),
		GetValidationService(),
		initModules.NewInstance[properties.JwtProp]().GetInstance(properties.NewJwtProp))
}

func AuthorizationUseCaseInstance() middlewareCore.AuthorizationUseCase[domain.User, *gin.Context] {
	return middlewareService.NewAuthorization[domain.User, *gin.Context](
		GetUserRepositoryInstance(),
		initModules.NewInstance[properties.JwtProp]().GetInstance(properties.NewJwtProp))
}

func UserUseCaseInstance() core.IUserUseCase {
	return services.GetUserServiceInstance(GetUserRepositoryInstance())
}
