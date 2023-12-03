package controller

import (
	"github.com/gin-gonic/gin"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/infra/primary/config"
	"sync"
)

var (
	testInstance *TestController
	testOnce     sync.Once
)

type TestController struct {
}

func GetTestControllerInstance() *TestController {
	testOnce.Do(func() {
		testInstance = &TestController{}
	})
	return testInstance
}

func (o *TestController) RunController(r *gin.Engine, authorizeMiddleware gin.HandlerFunc) {
	r.GET("/v1/test", func(c *gin.Context) {
		response, _ := config.GetToken("12345")

		c.JSON(200, response)
		//c.JSON(http.StatusNotFound, response)
	})

	// Grupo de rutas protegidas
	protected := r.Group("/v2")
	protected.Use(authorizeMiddleware)
	{
		protected.GET("/test", config.PoliciesGuard(funcName(), domain.USERS, domain.READ))
	}
}

func funcName() func(c *gin.Context) {
	return func(c *gin.Context) {
		response := "test2"
		c.JSON(200, response)
	}
}
