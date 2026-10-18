package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseServices "github.com/WilsonSayago/authBase/v4/core/services"
	"github.com/WilsonSayago/authBase/v4/infra/config/properties"
	"github.com/WilsonSayago/authBase/v4/infra/secundary"
	initModules "github.com/WilsonSayago/initModules/v2"
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/core/services"
	"github.com/gregperez/hachikapp-backend/internal/infra/audit"
	"github.com/gregperez/hachikapp-backend/internal/infra/config"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/appconfig"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/sql"
	applifecycle "github.com/gregperez/hachikapp-backend/internal/infra/lifecycle"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/ginadapter"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/oidcclient"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/oidccrypto"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/passwordcheck"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/ratelimit"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/auditstore"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/dao"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/oidcstore"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/petstore"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/refreshstore"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/settingsstore"
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
	auditPurger    *applifecycle.AuditEventPurger
	oidcPurger     *applifecycle.OIDCTransactionPurger
	oidcService    *services.OIDCService
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
	ownedDB := false
	if db == nil {
		opened, err := database.OpenPool(context.Background(), opts.Config.Database)
		if err != nil {
			return nil, fmt.Errorf("open database: %w", err)
		}
		db = opened
		ownedDB = true
	}
	released := false
	defer func() {
		if ownedDB && !released {
			db.Close()
		}
	}()

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
	released = true
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

	material, err := newTokenMaterial(opts.JWT, opts.Config.Auth.TokenKeys)
	if err != nil {
		return fmt.Errorf("token crypto: %w", err)
	}
	tokens, err := authBaseServices.NewTokenManagerWithCrypto(opts.JWT, material.Crypto)
	if err != nil {
		return fmt.Errorf("token manager: %w", err)
	}
	authn, err := authBaseServices.NewAuthenticationServiceWithCrypto[domain.User](
		users,
		users,
		refreshStore,
		validation,
		opts.JWT,
		material.Crypto,
	)
	if err != nil {
		return fmt.Errorf("authentication service: %w", err)
	}
	auditStore := auditstore.New(a.DB)
	settingsStore := settingsstore.New(a.DB)
	recorder, err := audit.NewRecorder(a.Logger, auditStore)
	if err != nil {
		return fmt.Errorf("audit recorder: %w", err)
	}
	authn.WithSecurityEvents(recorder)
	authz, err := authBaseServices.NewAuthorization[domain.User, *ginadapter.Context](users, tokens)
	if err != nil {
		return fmt.Errorf("authorization service: %w", err)
	}
	blocklist := passwordcheck.NewOffline(opts.Config.Auth.PasswordBlocklist)
	userUseCase := services.NewUserService(users, validation, recorder, blocklist)
	sessionUseCase := services.NewSessionService(refreshStore)
	auditUseCase := services.NewAuditService(auditStore)
	settingsUseCase := services.NewSettingsService(settingsStore, recorder)
	petUseCase := services.NewPetService(petstore.New(a.DB), petstore.NewPhotoStore(a.DB))
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
	if err := a.wireOIDC(opts, users, authn, recorder, recorder); err != nil {
		return err
	}

	translate := config.NewTranslate(a.Logger, opts.TranslationFiles...)
	gate := NewReadyGate(a.DB)
	engine := NewEngine(HTTPDeps{
		Config: opts.Config,
		Gate:   gate,
		Logger: a.Logger,
		JWKS:   material.JWKS,
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
				OIDC:           a.oidcService,
				OIDCConfig:     opts.Config.Auth.OIDC,
				SecureCookie:   opts.Config.IsProduction(),
				Sessions:       sessionUseCase,
				Audits:         auditUseCase,
				Settings:       settingsUseCase,
				Pets:           petUseCase,
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
		auditPurger, err := applifecycle.NewAuditEventPurger(auditStore, applifecycle.AuditEventPurgerConfig{
			Interval:   opts.Config.Auth.AuditPurgeInterval,
			Retention:  opts.Config.Auth.AuditRetention,
			MaxBatches: opts.Config.Auth.AuditPurgeMaxBatches,
		}, a.Logger)
		if err != nil {
			return fmt.Errorf("audit purger: %w", err)
		}
		a.auditPurger = auditPurger
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
	if a.auditPurger != nil {
		a.App.Register(a.auditPurger)
		a.lifecycleNames = append(a.lifecycleNames, "audit-purger")
	}
	if a.oidcPurger != nil {
		a.App.Register(a.oidcPurger)
		a.lifecycleNames = append(a.lifecycleNames, "oidc-purger")
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
	OIDC           *services.OIDCService
	OIDCConfig     appconfig.OIDCConfig
	SecureCookie   bool
	Sessions       core.SessionUseCase
	Audits         core.AuditUseCase
	Settings       core.SettingsUseCase
	Pets           core.PetUseCase
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
	controller.NewOIDCController(
		deps.OIDC,
		deps.Authz,
		r,
		deps.Translate,
		deps.Limiter,
		deps.OriginResolver,
		deps.OIDCConfig,
		deps.SecureCookie,
	).RunController()
	controller.NewSessionController(deps.Authz, deps.Sessions, deps.Tokens, r, deps.Translate).RunController()
	controller.NewAuditController(deps.Authz, deps.Audits, r, deps.Translate).RunController()
	controller.NewSettingsController(deps.Authz, deps.Settings, r, deps.Translate).RunController()
	controller.NewUserController(deps.Authz, deps.Users, r, deps.Translate).RunController()
	controller.NewRoleController(deps.Authz, deps.Roles, r, deps.Translate).RunController()
	controller.NewPetController(deps.Authz, deps.Pets, r, deps.Translate).RunController()
}

func (a *Application) wireOIDC(
	opts Options,
	users ports.UserPort,
	sessions authBaseCore.SessionIssuer,
	auditor audit.Logger,
	sink audit.SecuritySink,
) error {
	cfg := opts.Config.Auth.OIDC
	if !cfg.Enabled {
		return nil
	}
	key, err := cfg.EncryptionKeyBytes()
	if err != nil {
		return err
	}
	box, err := oidccrypto.NewBox(key)
	if err != nil {
		return fmt.Errorf("oidc encryption: %w", err)
	}
	store := oidcstore.New(a.DB)
	providers := make([]ports.OIDCProvider, 0, len(cfg.Providers))
	destinations := make(map[string][]string, len(cfg.Providers))
	for _, providerCfg := range cfg.Providers {
		client, err := oidcclient.New(providerCfg, cfg.HTTPTimeout, nil)
		if err != nil {
			return fmt.Errorf("oidc provider %s: %w", providerCfg.Name, err)
		}
		providers = append(providers, client)
		destinations[strings.ToLower(providerCfg.Name)] = append([]string(nil), providerCfg.AllowedFinalDestinations...)
	}
	validation := secundary.NewValidationService()
	service, err := services.NewOIDCService(
		services.OIDCServiceConfig{
			Enabled:        true,
			TransactionTTL: cfg.TransactionTTL,
			GrantTTL:       cfg.GrantTTL,
		},
		providers,
		destinations,
		store,
		store,
		users,
		box,
		sessions,
		validation,
		audit.FanoutOIDC{Inner: auditor, Sink: sink},
	)
	if err != nil {
		return fmt.Errorf("oidc service: %w", err)
	}
	a.oidcService = service
	if opts.SkipPurger {
		return nil
	}
	purger, err := applifecycle.NewOIDCTransactionPurger(store, applifecycle.OIDCTransactionPurgerConfig{
		Interval:   opts.Config.Auth.SessionPurgeInterval,
		Retention:  opts.Config.Auth.SessionPurgeRetention,
		MaxBatches: opts.Config.Auth.SessionPurgeMaxBatches,
	}, a.Logger)
	if err != nil {
		return fmt.Errorf("oidc purger: %w", err)
	}
	a.oidcPurger = purger
	return nil
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
