package instance

import (
	"github.com/WilsonSayago/initModules"
	"github.com/gin-gonic/gin"
	"jwt/base/internal/infra/primary/config/database"
	"jwt/base/internal/infra/primary/config/prop"
	"jwt/base/internal/infra/primary/controller"
)

func GetDatabasePersistencePgxInstance() initModules.IProcess {
	//return database.NewConnectionPostgresPgx(prop.GetDatabaseProperty())
	return database.NewConnectionPostgresPgx(initModules.NewInstance[prop.DatabaseProperty]().GetInstance(prop.NewDatabaseProperty))
}

func UserControllerInstance(r *gin.Engine) initModules.IProcess {
	return controller.GetUserControllerInstance(
		MiddlewareUseCaseInstance(),
		AuthenticationUseCaseInstance(),
		AuthorizationUseCaseInstance(),
		UserUseCaseInstance(),
		r)
}
