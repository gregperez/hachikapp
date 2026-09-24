#!/usr/bin/env bash
# Validate OpenAPI artifacts and fail on drift vs regenerated output.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# shellcheck disable=SC1091
source "${ROOT}/tools/versions.env"

GO="${GO:-go}"
GOWORK="${GOWORK:-off}"
export GOWORK

# Structural contract checks (Bearer, paths, no password in responses).
"${GO}" test -count=1 ./docs -run TestOpenAPIContract

# Drift: regenerate into an in-repo temp dir (swag resolves modules from ROOT).
tmpdir="${ROOT}/.tmp/openapi-regen"
rm -rf "$tmpdir"
mkdir -p "$tmpdir"
trap 'rm -rf "$tmpdir"' EXIT

mkdir -p "${ROOT}/.tmp/bin"
GOBIN="${ROOT}/.tmp/bin" go install "github.com/swaggo/swag/cmd/swag@${SWAG_VERSION}"

"${ROOT}/.tmp/bin/swag" init -g cmd/main.go -o "$tmpdir" --parseDependency --parseInternal
if ! diff -u docs/swagger.json "$tmpdir/swagger.json"; then
  echo "OpenAPI drift detected. Run: make openapi-generate" >&2
  exit 1
fi

echo "openapi-lint OK"
