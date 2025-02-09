package main

import (
	"jwt/base/internal/infra/config/instance"
	"jwt/base/internal/infra/config/prop"

	"github.com/WilsonSayago/authBase/infra/config/properties"
	"github.com/WilsonSayago/initModules"
	"github.com/gin-gonic/gin"
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

	instance.ExecuteMigration()

	r := gin.Default()

	initModules.RegisterProcess(instance.TranslateInstance())
	initModules.RegisterProcess(instance.GetDatabasePersistencePgxInstance())
	initModules.RegisterProcess(instance.AuthControllerInstance(r))
	initModules.RegisterProcess(instance.UserControllerInstance(r))
	initModules.RegisterProcess(instance.RoleControllerInstance(r))
	initModules.Init(true, true)

	r.Run()
}
