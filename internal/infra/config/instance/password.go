package instance

import (
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/passwordcheck"
	initModules "github.com/WilsonSayago/initModules/v2"
)

func PasswordBlocklistInstance(cfg appconfig.AuthConfig) domain.PasswordBlocklist {
	return initModules.Once(func() *passwordcheck.Offline {
		return passwordcheck.NewOffline(cfg.PasswordBlocklist)
	})
}
