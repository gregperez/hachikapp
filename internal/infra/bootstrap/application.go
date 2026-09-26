package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseServices "github.com/WilsonSayago/authBase/v4/core/services"
	"github.com/WilsonSayago/authBase/v4/infra/config/properties"
	"github.com/WilsonSayago/authBase/v4/infra/secundary"
	"github.com/WilsonSayago/base-golang/internal/core"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
	"github.com/WilsonSayago/base-golang/internal/core/services"
	"github.com/WilsonSayago/base-golang/internal/infra/audit"
	"github.com/WilsonSayago/base-golang/internal/infra/config"
	"github.com/WilsonSayago/base-golang/internal/infra/config/appconfig"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/config/sql"
	applifecycle "github.com/WilsonSayago/base-golang/internal/infra/lifecycle"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/ginadapter"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/passwordcheck"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/ratelimit"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/dao"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/refreshstore"
	initModules "github.com/WilsonSayago/initModules/v2"
	"github.com/gin-gonic/gin"
)

// Options configures one isolated Application instance.
type Options struct {
	Config           appconfig.Config
	JWT              properties.Jwt
	Logger           *slog.Logger
	DB               *database.PostgresPgx
	TranslationFiles []string
	SkipMigrate      bool
	SkipPurger       bool
	SkipListen       bool
	SkipDatabaseLife bool
}

// Application is the isolated composition root for one process unit.
type Application struct {
	Config    appconfig.Config
	Container *initModules.Container
	App       *initModules.App
	Logger    *slog.Logger
	DB        *database.PostgresPgx
	Engine    *gin.Engine
	Server    *http.Server
	Gate      *ReadyGate
	HTTP      *HTTPLifecycle
	Translate *config.Translate

	TokenManager *authBaseServices.TokenManager
	Users        ports.UserPort
	UserUseCase  core.UserUseCase

	refreshStore   *refreshstore.Store
	sessionPurger  *applifecycle.ExpiredSessionPurger
	lifecycleNames []string
}

// NewApplication wires one isolated process. It does not start goroutines,
// listen on ports, or mutate process-wide singletons.
func NewApplication(opts Options) (*Application, error) {
	if opts.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if err := opts.JWT.Validate(); err != nil {
		return nil, fmt.Errorf("invalid jwt configuration: %w", err)
	}
	if err := opts.Config.Validate(); err != nil {
		return nil, err
	}

	db := opts.DB
	if db == nil {
		opened, err := database.OpenPool(context.Background(), opts.Config.Database)
		if err != nil {
			return nil, fmt.Errorf("open database: %w", err)
		}
		db = opened
	}

	if !opts.SkipMigrate {
		dsn, err := opts.Config.Database.DSN()
		if err != nil {
			return nil, err
		}
		if err := sql.Migrate(opts.Config.Database.MigrationsPath, dsn); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}

	container := initModules.NewContainer()
	app := initModules.NewApp()
	application := &Application{
		Config:    opts.Config,
		Container: container,
		App:       app,
		Logger:    opts.Logger,
		DB:        db,
	}

	if err := application.wire(opts); err != nil {
		return nil, err
	}
	application.registerLifecycles(opts)
	return application, nil
}

func (a *Application) wire(opts Options) error {
	validation := secundary.NewValidationService()
	refreshStore := refreshstore.New(a.DB)
	users := repositories.NewUserRepository(
		dao.NewUserDao(a.DB),
		dao.NewRoleDao(a.DB),
		dao.NewUserRoleDao(a.DB),
		validation,
		refreshStore,
	)
	roles := repositories.NewRoleRepository(dao.NewRoleDao(a.DB))

	tokens, err := authBaseServices.NewTokenManager(opts.JWT)
	if err != nil {
		return fmt.Errorf("token manager: %w", err)
	}
	authn, err := authBaseServices.NewAuthenticationService[domain.User](
		users,
		users,
		refreshStore,
		validation,
		opts.JWT,
	)
	if err != nil {
		return fmt.Errorf("authentication service: %w", err)
	}
	authz, err := authBaseServices.NewAuthorization[domain.User, *ginadapter.Context](users, tokens)
	if err != nil {
		return fmt.Errorf("authorization service: %w", err)
	}
	auditor, err := audit.NewLogger(a.Logger)
	if err != nil {
		return err
	}
	blocklist := passwordcheck.NewOffline(opts.Config.Auth.PasswordBlocklist)
	userUseCase := services.NewUserService(users, validation, auditor, blocklist)
	roleUseCase := authBaseServices.GetRoleServiceInstance(roles)

	limiter, err := ratelimit.NewMemory(ratelimit.Config{
		Scopes: map[ports.LoginLimitScope]ratelimit.ScopeConfig{
			ports.LoginLimitAccount: {
				Limit:  opts.Config.Auth.LoginAccountLimit,
				Window: opts.Config.Auth.RateLimitWindow,
			},
			ports.LoginLimitOrigin: {
				Limit:  opts.Config.Auth.LoginOriginLimit,
				Window: opts.Config.Auth.RateLimitWindow,
			},
			ports.RefreshLimitFamily: {
				Limit:  opts.Config.Auth.RefreshFamilyLimit,
				Window: opts.Config.Auth.RateLimitWindow,
			},
		},
		MaxEntries: opts.Config.Auth.RateLimitMaxEntries,
	})
	if err != nil {
		return fmt.Errorf("login limiter: %w", err)
	}
	originResolver, err := controller.NewClientOriginResolver(opts.Config.Auth.TrustedProxyCIDRs)
	if err != nil {
		return fmt.Errorf("client origin resolver: %w", err)
	}

	translate := config.NewTranslate(a.Logger, opts.TranslationFiles...)
	gate := NewReadyGate(a.DB)
	engine := NewEngine(HTTPDeps{
		Config: opts.Config,
		Gate:   gate,
		Logger: a.Logger,
		Register: func(r *gin.Engine) {
			registerRoutes(r, routeDeps{
				Authn:          authn,
				Authz:          authz,
				Users:          userUseCase,
				Roles:          roleUseCase,
				Tokens:         tokens,
				Translate:      translate,
				Limiter:        limiter,
				OriginResolver: originResolver,
			})
		},
	})
	server := NewHTTPServer(opts.Config.Server, engine)

	a.Engine = engine
	a.Server = server
	a.Gate = gate
	a.Translate = translate
	a.TokenManager = tokens
	a.Users = users
	a.UserUseCase = userUseCase
	a.HTTP = NewHTTPLifecycle(server, gate, a.Logger)
	a.refreshStore = refreshStore
	if !opts.SkipPurger {
		purger, err := applifecycle.NewExpiredSessionPurger(refreshStore, applifecycle.ExpiredSessionPurgerConfig{
			Interval:   opts.Config.Auth.SessionPurgeInterval,
			Retention:  opts.Config.Auth.SessionPurgeRetention,
			MaxBatches: opts.Config.Auth.SessionPurgeMaxBatches,
		}, a.Logger)
		if err != nil {
			return fmt.Errorf("session purger: %w", err)
		}
		a.sessionPurger = purger
	}
	return nil
}

func (a *Application) registerLifecycles(opts Options) {
	if !opts.SkipDatabaseLife {
		a.App.Register(NewDatabaseLifecycle(a.DB, a.Logger))
		a.lifecycleNames = append(a.lifecycleNames, "database")
	}
	a.App.Register(NewTranslateLifecycle(a.Translate))
	a.lifecycleNames = append(a.lifecycleNames, "translations")
	if a.sessionPurger != nil {
		a.App.Register(a.sessionPurger)
		a.lifecycleNames = append(a.lifecycleNames, "session-purger")
	}
	if !opts.SkipListen {
		a.App.Register(a.HTTP)
		a.lifecycleNames = append(a.lifecycleNames, "http")
	}
}

// LifecycleNames reports registration order (start order; stop is the reverse).
func (a *Application) LifecycleNames() []string {
	return append([]string(nil), a.lifecycleNames...)
}

func (a *Application) Run(ctx context.Context) error {
	return a.App.RunContext(ctx, initModules.RunOptions{
		RunLifecycles: true,
		StopTimeout:   a.Config.Server.ShutdownTimeout,
	})
}

func (a *Application) RunWithSignals(ctx context.Context) error {
	return a.App.RunWithSignals(ctx, initModules.RunOptions{
		RunLifecycles: true,
		StopTimeout:   a.Config.Server.ShutdownTimeout,
	})
}

type routeDeps struct {
	Authn          authBaseCore.AuthenticationUseCase
	Authz          authBaseCore.AuthorizationUseCase[domain.User, *ginadapter.Context]
	Users          core.UserUseCase
	Roles          authBaseCore.RoleUseCase
	Tokens         *authBaseServices.TokenManager
	Translate      config.ITranslate
	Limiter        ports.LoginLimiter
	OriginResolver *controller.ClientOriginResolver
}

func registerRoutes(r *gin.Engine, deps routeDeps) {
	controller.NewAuthController(
		deps.Authn,
		deps.Authz,
		deps.Users,
		deps.Tokens,
		r,
		deps.Translate,
		deps.Limiter,
		deps.OriginResolver,
	).RunController()
	controller.NewUserController(deps.Authz, deps.Users, r, deps.Translate).RunController()
	controller.NewRoleController(deps.Authz, deps.Roles, r, deps.Translate).RunController()
}

// NewUserPersistence builds repositories for administrative tools without globals.
func NewUserPersistence(
	db *database.PostgresPgx,
	authCfg appconfig.AuthConfig,
) (ports.UserPort, domain.PasswordBlocklist, error) {
	if db == nil {
		return nil, nil, fmt.Errorf("database is required")
	}
	validation := secundary.NewValidationService()
	users := repositories.NewUserRepository(
		dao.NewUserDao(db),
		dao.NewRoleDao(db),
		dao.NewUserRoleDao(db),
		validation,
		refreshstore.New(db),
	)
	return users, passwordcheck.NewOffline(authCfg.PasswordBlocklist), nil
}
