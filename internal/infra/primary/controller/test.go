package controller

import (
	middlewareCore "github.com/WilsonSayago/middleware/core"
	middlewareDomain "github.com/WilsonSayago/middleware/core/domains"
	middlewareService "github.com/WilsonSayago/middleware/core/services"
	"github.com/gin-gonic/gin"
	"jwt/base/internal/core/domain"
	"net/http"
	"sync"
)

var (
	testInstance *TestController
	testOnce     sync.Once
)

type TestController struct {
	middleware middlewareCore.MiddlewareUseCase
	r          *gin.Engine
}

func GetTestControllerInstance(middleware middlewareCore.MiddlewareUseCase, r *gin.Engine) *TestController {
	testOnce.Do(func() {
		testInstance = &TestController{
			middleware: middleware,
			r:          r,
		}
	})
	return testInstance
}

func (o *TestController) Start() {
	o.RunController()
}

func (o *TestController) RunController() {
	(*o.r).GET("/v1/test", func(c *gin.Context) {
		response, _ := o.middleware.GetToken("1")
		//response, _ := middlewareService.GetToken("1")
		
		c.JSON(200, response)
		//c.JSON(http.StatusNotFound, response)
	})
	
	// Grupo de rutas protegidas
	protected := (*o.r).Group("/v2")
	protected.Use(o.middleware.AuthorizeJWT())
	{
		protected.GET("/test", middlewareService.PoliciesGuard(o.funcName(),
			middlewareDomain.USERS, middlewareDomain.READ))
		protected.POST("/test-post", middlewareService.PoliciesGuard(funcPost(),
			middlewareDomain.USERS, middlewareDomain.READ))
	}
}

func (o *TestController) funcName() func(c *gin.Context) {
	return func(c *gin.Context) {
		response := middlewareService.GetUserToken[domain.User](c)
		c.JSON(200, response)
	}
}

type RequestDTO struct {
	Name string `json:"name" binding:"required"`
	Age  int    `json:"age" binding:"required"`
}

func funcPost() func(c *gin.Context) {
	return func(c *gin.Context) {
		requestDTO := RequestDTO{}
		err := c.ShouldBind(&requestDTO)
		
		if err != nil {
			c.JSON(http.StatusBadRequest, err.Error())
			return
		}
		
		c.JSON(http.StatusAccepted, &requestDTO)
	}
}
