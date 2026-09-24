#!/usr/bin/env bash
# Regenerate OpenAPI/Swagger artifacts from Go annotations.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# shellcheck disable=SC1091
source "${ROOT}/tools/versions.env"

mkdir -p "${ROOT}/.tmp/bin"
export PATH="${ROOT}/.tmp/bin:${PATH}"

# Always install the pinned swag into .tmp/bin so PATH swag (e.g. v1.16) cannot
# generate docs incompatible with go.mod's github.com/swaggo/swag.
GOBIN="${ROOT}/.tmp/bin" go install "github.com/swaggo/swag/cmd/swag@${SWAG_VERSION}"

"${ROOT}/.tmp/bin/swag" init -g cmd/main.go -o docs --parseDependency --parseInternal
echo "openapi-generate OK"
