package main

import (
	_ "jwt/base/docs"
	"jwt/base/internal/infra/config/instance"
	"jwt/base/internal/infra/config/prop"

	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/WilsonSayago/authBase/infra/config/properties"
	"github.com/WilsonSayago/initModules"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
)

var (
	portEnv string
)

const (
	propertiesFile = "internal/resources/properties.yml"
)

// @title Mi API en Golang
// @version 1.0
// @description API de Autenticación con JWT en Go con Arquitectura Hexagonal
// @host localhost:8080
// @BasePath /v1/auth
func main() {

	initModules.SetFilePath(initModules.YML, propertiesFile)

	initModules.AddProp(initModules.NewInstance[prop.DatabaseProperty]().GetInstance(prop.NewDatabaseProperty))
	//initModules.AddProp(initModules.NewInstance[properties.MiddlewareProp]().GetInstance(properties.NewMiddlewareProp))
	initModules.AddProp(initModules.NewInstance[properties.JwtProp]().GetInstance(properties.NewJwtProp))

	initModules.RunLoadProperties()

	instance.ExecuteMigration()

	r := gin.Default()

	// Configuración de CORS
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:3000", "http://localhost:5173"} // Puertos comunes de desarrollo React
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "lang", "Accept"}
	config.ExposeHeaders = []string{"Content-Length", "Content-Type", "Authorization"}
	config.AllowCredentials = true
	config.MaxAge = 12 * 60 * 60 // 12 horas
	r.Use(cors.New(config))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	initModules.RegisterProcess(instance.TranslateInstance())
	initModules.RegisterProcess(instance.GetDatabasePersistencePgxInstance())
	initModules.RegisterProcess(instance.AuthControllerInstance(r))
	initModules.RegisterProcess(instance.UserControllerInstance(r))
	initModules.RegisterProcess(instance.RoleControllerInstance(r))
	initModules.Init(true, true)

	//r.Run(":8081")
	r.Run()
}
