package instance

import (
	"jwt/base/internal/infra/config"
	"jwt/base/internal/infra/config/database"
	"jwt/base/internal/infra/config/prop"
	"jwt/base/internal/infra/config/sql"
	"jwt/base/internal/infra/primary/controller"

	"github.com/WilsonSayago/initModules"
	"github.com/gin-gonic/gin"
)

func GetDatabasePersistencePgxInstance() initModules.IProcess {
	//return database.NewConnectionPostgresPgx(prop.GetDatabaseProperty())
	return database.NewConnectionPostgresPgx(initModules.NewInstance[prop.DatabaseProperty]().GetInstance(prop.NewDatabaseProperty))
}

func AuthControllerInstance(r *gin.Engine) initModules.IProcess {
	return controller.GetAuthControllerInstance(
		AuthenticationUseCaseInstance(),
		AuthorizationUseCaseInstance(),
		UserUseCaseInstance(),
		r,
		TranslateInstance())
}

func UserControllerInstance(r *gin.Engine) initModules.IProcess {
	return controller.GetUserControllerInstance(
		AuthenticationUseCaseInstance(),
		AuthorizationUseCaseInstance(),
		UserUseCaseInstance(),
		r,
		TranslateInstance())
}

func RoleControllerInstance(r *gin.Engine) initModules.IProcess {
	return controller.GetRoleControllerInstance(
		AuthorizationUseCaseInstance(),
		RoleUseCaseInstance(),
		r,
		TranslateInstance())
}

func ExecuteMigration() {
	sql.Migrate("internal/resources/sql/migrations")
}

func TranslateInstance() config.ITranslate {
	return config.GetTranslateInstance()
}
