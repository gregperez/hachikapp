# ADR 001 — Process lifecycle and initModules v2

## Status

Accepted (plan 005), superseded in part by plan 015

## Context

Plan 005 required an explicit startup/shutdown order: load and validate config,
open one PostgreSQL pool, migrate with the same DSN, register routes, serve HTTP
with timeouts, expose liveness/readiness, and shut down on signal with deadlines.

`initModules/v2` already provided `LoadProperties`, `Once`/`OnceValue`, and
`NewApp`/`Lifecycle`/`RunWithSignals`. The original `cmd/main.go` mixed those
APIs with process-wide singletons, so plan 005 kept an explicit bootstrap and
avoided the package-level App.

Plan 015 revisits that boundary now that isolated `NewContainer`, `OnceIn`,
`NewApp`, and `ConfigLoader` are the supported composition APIs.

## Decision

1. **Use** `initModules.NewConfigLoader` for YAML/env property loading. Do not
   register properties on the package-level loader.
2. **Use** one `initModules.NewApp` per `bootstrap.Application` and register
   `Lifecycle` adapters in dependency order: database, translations, expired
   session cleanup, optional OIDC transaction cleanup, then HTTP.
3. **Do not** use package-level `Once`, `SetDefault` loggers, or a database
   singleton on production paths. `OnceIn` is acceptable only with the
   Application's own container.
4. Migrations and the runtime pool still share one DSN from
   `appconfig.DatabaseConfig`.

## Consequences

- Startup failures (parse, ping, migrate, listen) abort before readiness.
- Shutdown marks readiness false, drains HTTP, stops workers, then closes the
  pool, using a stop context that is not the cancelled run context.
- Two Applications can be constructed in one process without sharing loggers,
  engines, or containers.
- Operators still configure server/CORS/Swagger via environment (`SERVER_*`,
  `CORS_ALLOWED_ORIGINS`, `SWAGGER_ENABLED`, `APP_ENV`, `DB_SSLMODE`,
  `DATABASE_URL`) in addition to the YAML template for DB/JWT secrets.
