package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	_ "github.com/WilsonSayago/base-golang/docs"
	"github.com/WilsonSayago/base-golang/internal/infra/config/instance"
	"github.com/WilsonSayago/base-golang/internal/infra/config/prop"

	"github.com/WilsonSayago/authBase/v3/infra/config/properties"
	initModules "github.com/WilsonSayago/initModules/v2"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const propertiesFile = "internal/resources/properties.yml"

// @title Mi API en Golang
// @version 1.0
// @description API de Autenticación con JWT en Go con Arquitectura Hexagonal
// @host localhost:8080
// @BasePath /v1/auth
func main() {
	dbProp := initModules.OnceValue(prop.NewDatabaseProperty)
	jwtProp := initModules.OnceValue(properties.NewJwtProp)

	if err := initModules.AddPropE(dbProp); err != nil {
		panic(fmt.Errorf("register database properties: %w", err))
	}
	if err := initModules.AddPropE(jwtProp); err != nil {
		panic(fmt.Errorf("register jwt properties: %w", err))
	}
	if err := initModules.LoadProperties(
		initModules.WithFilePath(propertiesFile),
		initModules.WithFormat(initModules.YML),
	); err != nil {
		panic(fmt.Errorf("load properties: %w", err))
	}
	if err := jwtProp.Validate(); err != nil {
		panic(fmt.Errorf("invalid jwt configuration: %w", err))
	}

	db := instance.OpenDatabase()
	instance.ExecuteMigration()

	r := gin.Default()
	corsCfg := cors.DefaultConfig()
	corsCfg.AllowOrigins = []string{"http://localhost:3000", "http://localhost:5173"}
	corsCfg.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	corsCfg.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "lang", "Accept"}
	corsCfg.ExposeHeaders = []string{"Content-Length", "Content-Type", "Authorization"}
	corsCfg.AllowCredentials = true
	corsCfg.MaxAge = 12 * time.Hour
	r.Use(cors.New(corsCfg))
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	tr := instance.TranslateInstance()
	tr.Start()
	instance.RegisterControllers(r)

	srv := &http.Server{Addr: ":8080", Handler: r}
	app := initModules.NewApp()
	app.Register(dbLifecycle{db: db})
	app.Register(httpLifecycle{server: srv})
	if err := app.RunWithSignals(context.Background(), initModules.RunOptions{
		LoadProperties: false,
		RunLifecycles:  true,
	}); err != nil {
		panic(err)
	}
}

type dbLifecycle struct {
	db interface{ Start() }
}

func (d dbLifecycle) Start(ctx context.Context) error {
	d.db.Start()
	return nil
}

func (d dbLifecycle) Stop(ctx context.Context) error {
	return nil
}

type httpLifecycle struct {
	server *http.Server
}

func (h httpLifecycle) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- h.server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (h httpLifecycle) Stop(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}
