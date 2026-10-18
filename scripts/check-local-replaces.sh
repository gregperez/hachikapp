#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODFILE="${1:-${ROOT}/go.mod}"

cd "${ROOT}"
go mod edit -json "${MODFILE}" | go run ./internal/tools/checklocalreplaces
