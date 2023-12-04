package main

import (
	"github.com/gin-gonic/gin"
	"jwt/base/internal/infra/primary/config"
	"jwt/base/internal/infra/primary/config/instance"
	"jwt/base/internal/infra/primary/config/prop"
	"jwt/base/internal/infra/primary/controller"
	"jwt/base/internal/initComponent"
)

var (
	portEnv string
)

const (
	propertiesFile = "internal/resources/properties.yml"
)

func main() {
	
	initComponent.SetFilePath(initComponent.YML, propertiesFile)
	initComponent.AddProp(prop.GetPostgresProp())
	initComponent.AddProp(prop.GetDatabaseMongoProperty())
	initComponent.AddProp(prop.GetDatabaseProperty())
	initComponent.RunLoadProperties()
	
	initComponent.RegisterPrimaryProcess(instance.GetDatabasePersistencePgxInstance())
	initComponent.RegisterPrimaryProcess(instance.GetDatabasePersistenceMongoInstance())
	initComponent.Init()
	
	r := gin.Default()
	
	controller.GetTestControllerInstance().RunController(r, config.AuthorizeJWT())
	
	r.Run()
}
