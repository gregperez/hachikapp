# Contributing

## Prerequisites

- Go toolchain from `go.mod` (`toolchain` directive)
- Docker (for the ephemeral Postgres used by DB gates)
- Optional local tools are installed into `.tmp/bin` by `make tools`

## Local quality gate

```sh
# Start an ephemeral test database (credentials only for local/CI tests)
docker compose -f docker-compose.test.yml up -d

export DATABASE_TEST_URL='postgres://postgres:test@localhost:54329/base_golang_test?sslmode=disable'

make verify
```

`make verify` runs fmt-check, lint, unit tests, race tests, secret scan, vulnerability scan, migration up/down/up, and DB integration tests.

## Unit vs integration

| Command | Needs DB? | Notes |
|---|---|---|
| `make test` | No | Fast unit tests under `./internal/...` |
| `make test-race` | No* | Same packages with `-race` (*refreshstore skips without a safe `DATABASE_TEST_URL`) |
| `make test-db` | Yes | Validates `DATABASE_TEST_URL`, migrates up→down→up, then DB tests |

`DATABASE_TEST_URL` must:

- use host `localhost`, `127.0.0.1`, `::1`, or `postgres`
- use a database name containing `test` (for example `base_golang_test`)

This guard rejects accidental use of shared/production databases. Helpers never print the DSN.

## Lint notes

`make lint` runs `go vet` and `staticcheck` (version pinned in `tools/versions.env`) with the
module toolchain. `.golangci.yml` documents the intended broader rule set; golangci-lint's
typecheck is not yet reliable against this Go toolchain, so it is not the blocking gate.

## Secrets

```sh
make secrets
```

Uses gitleaks with `.gitleaks.toml` and `.gitleaks-baseline.json` from plan 001.

## CI

GitHub Actions workflow `.github/workflows/ci.yml` mirrors `make verify` with an ephemeral Postgres service and pinned third-party actions (full commit SHAs).
