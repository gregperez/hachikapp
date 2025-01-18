package main

import (
	"github.com/WilsonSayago/authBase/infra/config/properties"
	"github.com/WilsonSayago/initModules"
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

	initModules.AddProp(initModules.NewInstance[prop.DatabaseProperty]().GetInstance(prop.NewDatabaseProperty))
	//initModules.AddProp(initModules.NewInstance[properties.MiddlewareProp]().GetInstance(properties.NewMiddlewareProp))
	initModules.AddProp(initModules.NewInstance[properties.JwtProp]().GetInstance(properties.NewJwtProp))

	initModules.RunLoadProperties()

	r := gin.Default()

	initModules.RegisterProcess(instance.GetDatabasePersistencePgxInstance())
	initModules.RegisterProcess(instance.UserControllerInstance(r))
	initModules.Init(true, true)

	r.Run()
}
