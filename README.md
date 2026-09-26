# base-golang

## Overview
Production-oriented Go API template with Gin, PostgreSQL, JWT
authentication/authorization, refresh-token rotation, cursor pagination, and
explicit startup/shutdown behavior.

## Libraries

### Go Modules
- **github.com/WilsonSayago/authBase/v4** ([v4.3.0](https://github.com/WilsonSayago/authBase/releases/tag/v4.3.0)): authentication, authorization, session lifecycle, Argon2id credentials, JWT key agility, and keyset pagination contracts.
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
- `GET /.well-known/jwks.json` — access public keys only, when Ed25519 is enabled

Process composition lives in `internal/infra/bootstrap.Application`. Each
instance owns its container, logger, pool, engine, and `initModules.App`.

Security operations (rotation, bootstrap contract, exposure response, secret scanning):

- [docs/security-operations.md](docs/security-operations.md)
- `./scripts/check-secrets.sh` (gitleaks)

Authentication throttling is enabled with bounded per-account, per-origin, and
per-refresh-family quotas. Forwarded client addresses are ignored unless
`AUTH_TRUSTED_PROXY_CIDRS` explicitly identifies the deployment's proxies.
Multi-replica deployments must replace the in-memory limiter with shared state.

Optional Google Sign-In is OpenID Connect, not the Gmail API. Enable it with
`OIDC_ENABLED=true` plus a 32-byte `OIDC_ENCRYPTION_KEY`, client credentials,
an exact HTTPS redirect URI, and an allowlist of frontend destinations.
Existing password users must link `(issuer, subject)` explicitly; matching
email alone does not create a session. The browser callback returns an opaque
one-time grant. `POST /v1/auth/oidc/google/exchange` then calls
`EstablishSession` and returns the existing token contract. Losing the last
authenticator (password plus every linked identity) requires a separate
account-recovery process.

JWT signing stays HMAC/HS256 unless `JWT_ALG=ed25519` is set with distinct
access/refresh key files (mode `0600`). During the cutover, set
`JWT_HMAC_OVERLAP=true` and wait at least the longest token lifetime before
removing the HMAC verifier. The process never follows `jku`/`x5u` from a
token. See [docs/security-operations.md](docs/security-operations.md).

**Auth note:** the historical pre-v3 → v3 migration invalidated older JWTs. The
current v4 upgrade changes pagination contracts and does not itself invalidate
v3 tokens when signing configuration remains unchanged. Refresh tokens rotate
atomically and are never stored in plaintext. Password changes and account
deactivation revoke every refresh family for that user in the same database
transaction as the sensitive account update.

## Dependencies

Go minimum **1.26.6** / preferred toolchain **1.27.1**. Update policy: [docs/dependency-policy.md](docs/dependency-policy.md).

The repository resolves `authBase/v4@v4.3.0` through the public Go module
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
