# base-golang

## Overview
Production-oriented Go API template with Gin, PostgreSQL, JWT
authentication/authorization, refresh-token rotation, cursor pagination, and
explicit startup/shutdown behavior.

## Libraries

### Go Modules
- **github.com/WilsonSayago/authBase/v4** ([v4.0.1](https://github.com/WilsonSayago/authBase/releases/tag/v4.0.1)): authentication, authorization, and keyset pagination contracts.
- **github.com/WilsonSayago/initModules/v2** ([v2.0.0](https://github.com/WilsonSayago/initModules/releases/tag/v2.0.0)): config load and typed singletons.
- Logging uses Go's standard `log/slog` (JSON to stdout).

## Running PostgreSQL with Docker
To run a PostgreSQL instance using Docker, use a local password of your choice (do not reuse production secrets):

```sh
docker run --name base_golang -e POSTGRES_PASSWORD=change-me-local-only -d -p 5432:5432 postgres
```

### Internal Packages
- **internal/core**: domain, application ports, and use cases.
- **internal/infra/primary**: Gin controllers, DTOs, middleware, and HTTP errors.
- **internal/infra/secondary/repositories**: port implementations and PostgreSQL adapters.
- **internal/infra/secondary/repositories/postgres/dao**: SQL-focused data access.
- **internal/infra/bootstrap**: HTTP engine, health probes, signals, and graceful shutdown.

Architecture details and interactive diagrams:

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- [docs/diagrams/](docs/diagrams/)

## Database
The project uses PostgreSQL as the primary database. The database schema includes a `users` table with the following fields:
- `name`
- `password`
- `email`
- `is_admin`
- `active`
- database-managed `created_at` / `updated_at`

Migrations create schema and catalog data. A historical seed in `000002` is revoked by forward migration `000006` so installs do not keep a known privileged account. Bootstrap a real admin via the procedure in [docs/security-operations.md](docs/security-operations.md).

The current schema also contains `roles`, `users_roles`, and
`refresh_sessions`; see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and the
versioned migrations for ownership and lifecycle details.

## Migrations
Database migrations are managed using SQL files under `internal/resources/sql/migrations`.
Create a new pair with a descriptive name (replace `<descriptive_name>`) and fill in both files before committing:

```sh
  migrate create -ext sql -dir internal/resources/sql/migrations -seq <descriptive_name>
```

## Configuration
Runtime configuration is loaded from `internal/resources/properties.yml` (gitignored)
plus environment overrides. Start from the tracked template:

```sh
cp internal/resources/properties.example.yml internal/resources/properties.yml
cp .env.example .env
# set DB_PASSWORD, JWT_ACCESS_SECRET, JWT_REFRESH_SECRET (see .env.example)
```

Process lifecycle (shared DSN for migrate + pool, health probes, graceful shutdown):
see [docs/adr/001-initmodules-lifecycle.md](docs/adr/001-initmodules-lifecycle.md).

Health (unversioned):

- `GET /health/live` — process up
- `GET /health/ready` — startup complete and DB reachable

Instance wiring is split by responsibility under
`internal/infra/config/instance/`: `port.go` (repositories/adapters),
`usecase.go` (authBase and application services), and `primary.go`
(controllers/routes).

Security operations (rotation, bootstrap contract, exposure response, secret scanning):

- [docs/security-operations.md](docs/security-operations.md)
- `./scripts/check-secrets.sh` (gitleaks)

**Auth note:** the historical pre-v3 → v3 migration invalidated older JWTs. The
current v4 upgrade changes pagination contracts and does not itself invalidate
v3 tokens when signing configuration remains unchanged. Refresh tokens rotate
atomically and are never stored in plaintext.

## Dependencies

Go minimum **1.26.6** / preferred toolchain **1.27.1**. Update policy: [docs/dependency-policy.md](docs/dependency-policy.md).

The repository resolves `authBase/v4@v4.0.1` through the public Go module
proxy. Checked-in filesystem `replace` directives are rejected by
`make mod-policy`, so a standalone checkout does not depend on sibling
repositories.

## Development quality gate

See [CONTRIBUTING.md](CONTRIBUTING.md). Short version:

```sh
docker compose -f docker-compose.test.yml up -d
export DATABASE_TEST_URL='postgres://postgres:test@localhost:54329/base_golang_test?sslmode=disable'
make verify
```

## Usage
To run the project, ensure you have Go installed and set up your PostgreSQL database. Then, use the following commands:

```sh
  go mod tidy
  go run ./cmd
```

## Generate API documentation with Swagger (Swaggo)
You can test the generated documentation by visiting [http://localhost:8080/swagger/index.html#/](http://localhost:8080/swagger/index.html#/).

Regenerate and verify the committed contract with:

```sh
make openapi-generate
make openapi-lint
```

Contract changes are recorded in [docs/CHANGELOG.md](docs/CHANGELOG.md).
