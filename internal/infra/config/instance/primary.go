package instance

import (
	"fmt"

	"github.com/WilsonSayago/base-golang/internal/infra/config"
	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/config/prop"
	"github.com/WilsonSayago/base-golang/internal/infra/config/sql"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller"
	initModules "github.com/WilsonSayago/initModules/v2"
	"github.com/gin-gonic/gin"
)

func OpenDatabase() *database.PostgresPgx {
	dbProp := initModules.OnceValue(prop.NewDatabaseProperty)
	db := database.NewConnectionPostgresPgx(dbProp)
	if db == nil {
		panic(fmt.Errorf("open database: %w", database.OpenError()))
	}
	return db
}

func RegisterControllers(r *gin.Engine, cfg appconfig.Config) {
	controller.GetAuthControllerInstance(
		AuthenticationUseCaseInstance(),
		AuthorizationUseCaseInstance(),
		UserUseCaseInstance(cfg.Auth),
		TokenManagerInstance(),
		r,
		TranslateInstance(),
		LoginLimiterInstance(cfg.Auth),
		ClientOriginResolverInstance(cfg.Auth),
	).RunController()

	controller.GetUserControllerInstance(
		AuthorizationUseCaseInstance(),
		UserUseCaseInstance(cfg.Auth),
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

func ExecuteMigration(dsn string) error {
	return sql.Migrate("internal/resources/sql/migrations", dsn)
}

func TranslateInstance() config.ITranslate {
	return config.GetTranslateInstance()
}
