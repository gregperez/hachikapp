package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/WilsonSayago/base-golang/docs"
	"github.com/WilsonSayago/base-golang/internal/infra/bootstrap"
	"github.com/WilsonSayago/base-golang/internal/infra/logging"
)

const propertiesFile = "internal/resources/properties.yml"

// @title Mi API en Golang
// @version 1.0
// @description API de Autenticación con JWT en Go con Arquitectura Hexagonal. Private routes require Authorization: Bearer <access_token>.
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT access token. Value must be: Bearer <token>
func main() {
	bootLog := logging.New(os.Getenv("LOG_LEVEL"))
	if err := run(); err != nil {
		bootLog.Error("application stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, jwt, err := bootstrap.LoadRuntimeInputs(propertiesFile)
	if err != nil {
		return err
	}
	logger := logging.New(cfg.Runtime.LogLevel)
	if logger == nil {
		return fmt.Errorf("logger is required")
	}

	app, err := bootstrap.NewApplication(bootstrap.Options{
		Config: cfg,
		JWT:    jwt,
		Logger: logger,
	})
	if err != nil {
		return err
	}
	return app.RunWithSignals(context.Background())
}
