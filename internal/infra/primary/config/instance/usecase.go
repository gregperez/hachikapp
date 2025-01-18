package instance

import (
	authBaseCore "github.com/WilsonSayago/authBase/core"
	middlewareService "github.com/WilsonSayago/authBase/core/services"
	"github.com/WilsonSayago/authBase/infra/config/properties"
	"github.com/WilsonSayago/initModules"
	"github.com/gin-gonic/gin"
	"jwt/base/internal/core"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/core/services"
)

func AuthenticationUseCaseInstance() authBaseCore.AuthenticationUseCase {
	return middlewareService.GetAuthenticationInstance[domain.User](
		GetUserRepositoryInstance(),
		GetValidationService(),
		initModules.NewInstance[properties.JwtProp]().GetInstance(properties.NewJwtProp))
}

func AuthorizationUseCaseInstance() authBaseCore.AuthorizationUseCase[domain.User, *gin.Context] {
	return middlewareService.NewAuthorization[domain.User, *gin.Context](
		GetUserRepositoryInstance(),
		initModules.NewInstance[properties.JwtProp]().GetInstance(properties.NewJwtProp))
}

func UserUseCaseInstance() core.IUserUseCase {
	return services.GetUserServiceInstance(GetUserRepositoryInstance())
}
