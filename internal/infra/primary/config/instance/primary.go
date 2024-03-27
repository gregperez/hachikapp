package instance

import (
	"github.com/WilsonSayago/initModules"
	"github.com/gin-gonic/gin"
	"jwt/base/internal/infra/primary/config/database"
	"jwt/base/internal/infra/primary/config/prop"
	"jwt/base/internal/infra/primary/controller"
	subscription "jwt/base/internal/infra/primary/subscription/rabbitmq"
)

func GetDatabasePersistencePgxInstance() initModules.IProcess {
	return database.NewConnectionPostgresPgx(prop.GetDatabaseProperty())
}

func GetDatabasePersistenceMongoInstance() initModules.IProcess {
	return database.NewConnectionMongo(prop.GetDatabaseMongoProperty())
}

func TestControllerInstance(r *gin.Engine) initModules.IProcess {
	return controller.GetTestControllerInstance(MiddlewareUseCaseInstance(), r)
}

func TestSubscription() initModules.IProcess {
	return subscription.NewTestSubscription()
}
