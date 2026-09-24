# ADR 001 — Process lifecycle and initModules v2

## Status

Accepted (plan 005)

## Context

Plan 005 requires an explicit startup/shutdown order: load and validate config,
open one PostgreSQL pool, migrate with the same DSN, register routes, serve HTTP
with timeouts, expose liveness/readiness, and shut down on signal with deadlines.

`initModules/v2` already provides `LoadProperties`, `Once`/`OnceValue` for
singleton wiring, and `NewApp`/`Lifecycle`/`RunWithSignals`. The previous
`cmd/main.go` used `NewApp` but blocked inside HTTP `Start` until cancel, which
mis-handled errors and never reliably called `Shutdown` via `Stop`.

## Decision

1. **Keep** `initModules/v2` for YAML/env property loading and `Once` DI wiring
   used throughout `internal/infra/config/instance`.
2. **Do not** drive HTTP/DB process lifecycle through `initModules.NewApp` for
   this service. Use an explicit bootstrap in `cmd/main.go` plus
   `internal/infra/bootstrap` (`RunServer`, health gate, engine construction).
3. Migrations and the runtime pool share one DSN from `appconfig.DatabaseConfig`.

## Consequences

- Startup failures (parse, ping, migrate) abort before listening.
- Shutdown marks readiness false, calls `http.Server.Shutdown` with a deadline,
  then closes the pool.
- Operators configure server/CORS/Swagger via environment (`SERVER_*`,
  `CORS_ALLOWED_ORIGINS`, `SWAGGER_ENABLED`, `APP_ENV`, `DB_SSLMODE`,
  `DATABASE_URL`) in addition to the YAML template for DB/JWT secrets.
- Removing `initModules` entirely remains possible later; this ADR only pins the
  lifecycle boundary.
