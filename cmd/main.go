package main

import (
	"github.com/gin-gonic/gin"
	"jwt/base/internal/infra/primary/config"
	"jwt/base/internal/infra/primary/controller"
)

var (
	portEnv string
)

func main() {

	r := gin.Default()

	controller.GetTestControllerInstance().RunController(r, config.AuthorizeJWT())

	r.Run()
}
