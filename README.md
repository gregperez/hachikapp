# Project Name

## Overview
This project is a Go-based application that uses PostgreSQL for data storage and includes various libraries for handling authentication, database operations, and more.

## Libraries

### Go Modules
- **github.com/WilsonSayago/authBase**: A library for handling authentication and authorization.
- **github.com/sirupsen/logrus**: A structured logger for Go.
- **github.com/WilsonSayago/initModules**: A library for initializing modules in a Go application.

## Running PostgreSQL with Docker
To run a PostgreSQL instance using Docker, use a local password of your choice (do not reuse production secrets):

```sh
docker run --name base_golang -e POSTGRES_PASSWORD=change-me-local-only -d -p 5432:5432 postgres
```

### Internal Packages
- **jwt/base/internal/core/ports**: Contains the core ports/interfaces for the application.
- **jwt/base/internal/infra/config/database**: Handles database configuration and connections.
- **jwt/base/internal/infra/secondary/repositories**: Contains repository implementations for data access.
- **jwt/base/internal/infra/secondary/repositories/postgres/dao**: Data access objects for PostgreSQL.

## Database
The project uses PostgreSQL as the primary database. The database schema includes a `users` table with the following fields:
- `name`
- `password`
- `email`
- `is_admin`

Migrations create schema and catalog data. A historical seed in `000002` is revoked by forward migration `000006` so installs do not keep a known privileged account. Bootstrap a real admin via the procedure in [docs/security-operations.md](docs/security-operations.md).

## Migrations
Database migrations are managed using SQL files under `internal/resources/sql/migrations`.

```sh
  migrate create -ext sql -dir internal/resources/sql/migrations -seq insert_users_table
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

Instance wiring (repositories, validation, controllers) lives in `internal/infra/config/instance/port.go`.

Security operations (rotation, bootstrap contract, exposure response, secret scanning):

- [docs/security-operations.md](docs/security-operations.md)
- `./scripts/check-secrets.sh` (gitleaks)

**Auth note:** after upgrading to authBase v3, all prior JWTs are invalid; users must sign in again. Refresh tokens rotate atomically and are never stored in plaintext.

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

## Generate Documentation with Swagger (Swaggo) 
You can test the generated documentation by visiting [http://localhost:8080/swagger/index.html#/](http://localhost:8080/swagger/index.html#/).

To generate documentation using Swagger (Swaggo), use the following command:

```sh 
  swag init -d ./cmd,./internal/infra/primary/controller,./internal/infra/primary/controller/dto 
```
