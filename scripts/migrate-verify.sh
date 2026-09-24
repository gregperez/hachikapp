#!/usr/bin/env bash
# Apply migrations up, down to zero, then up again against DATABASE_TEST_URL.
# Does not print the DSN.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# shellcheck disable=SC1091
source "${ROOT}/tools/versions.env"

if [[ -z "${DATABASE_TEST_URL:-}" ]]; then
  echo "DATABASE_TEST_URL is required" >&2
  exit 1
fi

# Validate via Go helper without echoing the DSN.
go run ./internal/test/testdb/cmd/checkdsn >/dev/null

MIGRATE_PKG="github.com/golang-migrate/migrate/v4/cmd/migrate@${MIGRATE_VERSION}"

echo "migrate: up"
go run -tags 'postgres' "${MIGRATE_PKG}" -path internal/resources/sql/migrations -database "${DATABASE_TEST_URL}" up

echo "migrate: down (to version 0)"
go run -tags 'postgres' "${MIGRATE_PKG}" -path internal/resources/sql/migrations -database "${DATABASE_TEST_URL}" down -all

echo "migrate: up (again)"
go run -tags 'postgres' "${MIGRATE_PKG}" -path internal/resources/sql/migrations -database "${DATABASE_TEST_URL}" up

echo "migrate verify OK"
