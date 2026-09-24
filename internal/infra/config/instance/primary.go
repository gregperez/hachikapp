package instance

import (
	"github.com/WilsonSayago/base-golang/internal/infra/config"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/config/prop"
	"github.com/WilsonSayago/base-golang/internal/infra/config/sql"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller"
	initModules "github.com/WilsonSayago/initModules/v2"
	"github.com/gin-gonic/gin"
)

func OpenDatabase() *database.PostgresPgx {
	dbProp := initModules.OnceValue(prop.NewDatabaseProperty)
	return database.NewConnectionPostgresPgx(dbProp)
}

func RegisterControllers(r *gin.Engine) {
	controller.GetAuthControllerInstance(
		AuthenticationUseCaseInstance(),
		AuthorizationUseCaseInstance(),
		UserUseCaseInstance(),
		TokenManagerInstance(),
		r,
		TranslateInstance(),
	).RunController()

	controller.GetUserControllerInstance(
		AuthorizationUseCaseInstance(),
		UserUseCaseInstance(),
		r,
		TranslateInstance(),
	).RunController()

	controller.GetRoleControllerInstance(
		AuthorizationUseCaseInstance(),
		RoleUseCaseInstance(),
		r,
		TranslateInstance(),
	).RunController()
}

func ExecuteMigration() {
	sql.Migrate("internal/resources/sql/migrations")
}

func TranslateInstance() config.ITranslate {
	return config.GetTranslateInstance()
}
