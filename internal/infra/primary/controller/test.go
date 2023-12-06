package controller

import (
	"github.com/gin-gonic/gin"
	"jwt/base/internal/core"
	"jwt/base/internal/core/domain"
	"jwt/base/internal/core/services"
	"sync"
)

var (
	testInstance *TestController
	testOnce     sync.Once
)

type TestController struct {
	middleware *core.MiddlewareUseCase
	r          *gin.Engine
}

func GetTestControllerInstance(middleware *core.MiddlewareUseCase, r *gin.Engine) *TestController {
	testOnce.Do(func() {
		testInstance = &TestController{
			middleware: middleware,
			r:          r,
		}
	})
	return testInstance
}

func (db *TestController) Start() {
	db.RunController()
}

func (o *TestController) RunController() {
	(*o.r).GET("/v1/test", func(c *gin.Context) {
		response, _ := services.GetToken("1")

		c.JSON(200, response)
		//c.JSON(http.StatusNotFound, response)
	})

	// Grupo de rutas protegidas
	protected := (*o.r).Group("/v2")
	protected.Use((*o.middleware).AuthorizeJWT())
	{
		protected.GET("/test", services.PoliciesGuard(funcName(), domain.USERS, domain.READ))
	}
}

func funcName() func(c *gin.Context) {
	return func(c *gin.Context) {
		response := "test2"
		c.JSON(200, response)
	}
}
