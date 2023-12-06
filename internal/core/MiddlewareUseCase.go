package core

import "github.com/gin-gonic/gin"

type MiddlewareUseCase interface {
	AuthorizeJWT() gin.HandlerFunc
}
