package instance

import (
	"fmt"

	authBaseCore "github.com/WilsonSayago/authBase/v3/core"
	authBaseServices "github.com/WilsonSayago/authBase/v3/core/services"
	"github.com/WilsonSayago/authBase/v3/infra/config/properties"
	"github.com/WilsonSayago/base-golang/internal/core"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/services"
	"github.com/WilsonSayago/base-golang/internal/infra/audit"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/ginadapter"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/refreshstore"
	initModules "github.com/WilsonSayago/initModules/v2"
)

func jwtProp() *properties.JwtProp {
	return initModules.OnceValue(properties.NewJwtProp)
}

func TokenManagerInstance() *authBaseServices.TokenManager {
	return initModules.Once(func() *authBaseServices.TokenManager {
		tm, err := authBaseServices.NewTokenManager(jwtProp().Jwt)
		if err != nil {
			panic(fmt.Errorf("token manager: %w", err))
		}
		return tm
	})
}

func RefreshTokenStoreInstance() *refreshstore.Store {
	return initModules.Once(func() *refreshstore.Store {
		return refreshstore.New(database.GetDatabasePostgresPgx())
	})
}

func AuthenticationUseCaseInstance() authBaseCore.AuthenticationUseCase {
	return initModules.Once(func() *authBaseServices.AuthenticationService[domain.User] {
		svc, err := authBaseServices.NewAuthenticationService[domain.User](
			GetUserRepositoryInstance(),
			GetUserRepositoryInstance(),
			RefreshTokenStoreInstance(),
			GetValidationService(),
			jwtProp().Jwt,
		)
		if err != nil {
			panic(fmt.Errorf("authentication service: %w", err))
		}
		return svc
	})
}

func AuthorizationUseCaseInstance() authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context] {
	return initModules.Once(func() *authBaseServices.Authorization[domain.User, *ginadapter.Context] {
		authz, err := authBaseServices.NewAuthorization[domain.User, *ginadapter.Context](
			GetUserRepositoryInstance(),
			TokenManagerInstance(),
		)
		if err != nil {
			panic(fmt.Errorf("authorization service: %w", err))
		}
		return authz
	})
}

func UserUseCaseInstance() core.UserUseCase {
	return services.GetUserServiceInstance(
		GetUserRepositoryInstance(),
		GetValidationService(),
		audit.NewLogger(nil),
	)
}

func RoleUseCaseInstance() authBaseCore.RoleUseCase {
	return authBaseServices.GetRoleServiceInstance(GetRoleRepositoryInstance())
}
