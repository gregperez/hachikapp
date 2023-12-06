package instance

import (
	"github.com/gin-gonic/gin"
	"jwt/base/internal/infra/primary/config/database"
	"jwt/base/internal/infra/primary/config/prop"
	"jwt/base/internal/infra/primary/controller"
	"jwt/base/internal/initComponent"
)

func GetDatabasePersistencePgxInstance() initComponent.IPrimaryProcess {
	return database.NewConnectionPostgresPgx(prop.GetDatabaseProperty())
}

func GetDatabasePersistenceMongoInstance() initComponent.IPrimaryProcess {
	return database.NewConnectionMongo(prop.GetDatabaseMongoProperty())
}

func TestControllerInstance(r *gin.Engine) initComponent.IPrimaryProcess {
	return controller.GetTestControllerInstance(MiddlewareUseCaseInstance(), r)
}
