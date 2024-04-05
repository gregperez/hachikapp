package main

import (
	"github.com/WilsonSayago/initModules"
	"github.com/WilsonSayago/middleware/infra/config/properties"
	"github.com/gin-gonic/gin"
	"jwt/base/internal/infra/primary/config/instance"
	"jwt/base/internal/infra/primary/config/prop"
)

var (
	portEnv string
)

const (
	propertiesFile = "internal/resources/properties.yml"
)

func main() {
	
	initModules.SetFilePath(initModules.YML, propertiesFile)
	initModules.AddProp(prop.GetQueueProp())
	initModules.AddProp(prop.GetDatabaseMongoProperty())
	initModules.AddProp(prop.GetDatabaseProperty())
	initModules.AddProp(properties.GetMiddlewareProp())
	initModules.RunLoadProperties()
	
	r := gin.Default()
	
	initModules.RegisterProcess(instance.GetDatabasePersistencePgxInstance())
	initModules.RegisterProcess(instance.GetDatabasePersistenceMongoInstance())
	initModules.RegisterProcess(instance.TestControllerInstance(r))
	initModules.RegisterProcess(instance.TestSubscription())
	initModules.Init(true, true)
	
	//controller.GetTestControllerInstance().RunController(r, config.AuthorizeJWT())
	
	r.Run()
}
