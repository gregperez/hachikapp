package bootstrap

import (
	"fmt"

	"github.com/WilsonSayago/authBase/v4/infra/config/properties"
	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/WilsonSayago/base-golang/internal/infra/config/prop"
	initModules "github.com/WilsonSayago/initModules/v2"
)

const DefaultPropertiesFile = "internal/resources/properties.yml"

// LoadRuntimeInputs loads YAML properties through an isolated ConfigLoader and
// validates runtime configuration. It does not touch the global property registry.
func LoadRuntimeInputs(propertiesFile string) (appconfig.Config, properties.Jwt, error) {
	if propertiesFile == "" {
		propertiesFile = DefaultPropertiesFile
	}
	dbProp := prop.NewDatabaseProperty()
	jwtProp := properties.NewJwtProp()
	loader := initModules.NewConfigLoader(
		initModules.WithFilePath(propertiesFile),
		initModules.WithFormat(initModules.YML),
	)
	if err := loader.AddProp(&dbProp); err != nil {
		return appconfig.Config{}, properties.Jwt{}, fmt.Errorf("register database properties: %w", err)
	}
	if err := loader.AddProp(&jwtProp); err != nil {
		return appconfig.Config{}, properties.Jwt{}, fmt.Errorf("register jwt properties: %w", err)
	}
	if err := loader.Load(); err != nil {
		return appconfig.Config{}, properties.Jwt{}, fmt.Errorf("load properties: %w", err)
	}
	if err := jwtProp.Validate(); err != nil {
		return appconfig.Config{}, properties.Jwt{}, fmt.Errorf("invalid jwt configuration: %w", err)
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
		return appconfig.Config{}, properties.Jwt{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, jwtProp.Jwt, nil
}
