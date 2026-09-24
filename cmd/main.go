package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/WilsonSayago/base-golang/docs"
	"github.com/WilsonSayago/base-golang/internal/infra/bootstrap"
	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/config/instance"
	"github.com/WilsonSayago/base-golang/internal/infra/config/prop"
	"github.com/WilsonSayago/base-golang/internal/infra/config/sql"

	"github.com/WilsonSayago/authBase/v3/infra/config/properties"
	initModules "github.com/WilsonSayago/initModules/v2"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const propertiesFile = "internal/resources/properties.yml"

// @title Mi API en Golang
// @version 1.0
// @description API de Autenticación con JWT en Go con Arquitectura Hexagonal
// @host localhost:8080
// @BasePath /v1/auth
func main() {
	if err := run(); err != nil {
		log.Errorf("application stopped: %v", err)
		os.Exit(1)
	}
}

func run() error {
	dbProp := initModules.OnceValue(prop.NewDatabaseProperty)
	jwtProp := initModules.OnceValue(properties.NewJwtProp)

	if err := initModules.AddPropE(dbProp); err != nil {
		return fmt.Errorf("register database properties: %w", err)
	}
	if err := initModules.AddPropE(jwtProp); err != nil {
		return fmt.Errorf("register jwt properties: %w", err)
	}
	if err := initModules.LoadProperties(
		initModules.WithFilePath(propertiesFile),
		initModules.WithFormat(initModules.YML),
	); err != nil {
		return fmt.Errorf("load properties: %w", err)
	}
	if err := jwtProp.Validate(); err != nil {
		return fmt.Errorf("invalid jwt configuration: %w", err)
	}

	ssl := dbProp.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	cfg, err := appconfig.Load(appconfig.DatabaseConfig{
		Host:     dbProp.Host,
		Port:     dbProp.Port,
		Name:     dbProp.Name,
		User:     dbProp.User,
		Password: dbProp.Password,
		SSLMode:  ssl,
		MaxConns: dbProp.DbMaxConns,
	})
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	applyLogLevel(cfg.Runtime.LogLevel)

	dsn, err := cfg.Database.DSN()
	if err != nil {
		return err
	}

	ctx := context.Background()
	db, err := database.OpenPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	database.SetSingleton(db)
	defer db.Close()

	if err := sql.Migrate(cfg.Database.MigrationsPath, dsn); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	tr := instance.TranslateInstance()
	tr.Start()

	gate := bootstrap.NewReadyGate(db)
	engine := bootstrap.NewEngine(bootstrap.HTTPDeps{
		Config: cfg,
		Gate:   gate,
		Register: func(r *gin.Engine) {
			instance.RegisterControllers(r)
		},
	})

	srv := bootstrap.NewHTTPServer(cfg.Server, engine)
	gate.SetReady(true)

	runCtx, stop := bootstrap.SignalContext(ctx)
	defer stop()

	log.WithField("addr", cfg.Server.Address).Info("http listening")
	if err := bootstrap.RunServer(runCtx, srv, gate, cfg.Server.ShutdownTimeout); err != nil {
		return err
	}
	log.Info("shutdown complete")
	return nil
}

func applyLogLevel(level string) {
	parsed, err := log.ParseLevel(level)
	if err != nil {
		parsed = log.InfoLevel
	}
	log.SetLevel(parsed)
	log.SetFormatter(&log.JSONFormatter{})
}
