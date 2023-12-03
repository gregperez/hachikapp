package config

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"jwt/base/internal/core/domain"
	"net/http"
	"time"
)

// SecretKey usado para firmar los tokens
var SecretKey = []byte("tuSecretKey")

type MyCustomClaims struct {
	User string `json:"user"`
	jwt.RegisteredClaims
}

func AuthorizeJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		const Bearer_schema = "Bearer "
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "No se proporcionó el token de autorización"})
			return
		}

		tokenString := authHeader[len(Bearer_schema):]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("método de firma inesperado: %v", token.Header["alg"])
			}
			return SecretKey, nil
		})

		if token != nil && token.Valid {
			user := domain.User{
				Id:      "1",
				Email:   "test",
				IsAdmin: false,
				Permissions: []domain.Permission{
					{
						Entity: domain.USERS,
						Create: false,
						Read:   true,
						Update: true,
						Delete: true,
					},
				},
			}
			c.Set("user", &user)
			c.Next()
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		}
	}
}

func GetToken(userId string) (string, error) {
	claims := MyCustomClaims{
		userId,
		jwt.RegisteredClaims{
			// A usual scenario is to set the expiration time relative to the current time
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			//IssuedAt:  jwt.NewNumericDate(time.Now()),
			//NotBefore: jwt.NewNumericDate(time.Now()),
			//Issuer:    "test",
			//Subject:   "somebody",
			//ID:        "1",
			//Audience:  []string{"somebody_else"},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(SecretKey)

	if err != nil {
		fmt.Println(err)
		return "", err
	}
	return ss, nil
}

func PoliciesGuard(fn gin.HandlerFunc, entity domain.EntityEnum, operation domain.OperationEnum) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetUserToken(c)
		if user == nil || (!user.IsAdmin && !user.HasPermission(entity, operation)) {
			c.Status(http.StatusUnauthorized)
			return
		}
		fn(c)
	}
}

func GetUserToken(c *gin.Context) *domain.User {
	user, exist := c.Get("user")
	if !exist {
		return nil
	}
	return user.(*domain.User)
}
