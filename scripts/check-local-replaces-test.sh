#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHECK="${ROOT}/scripts/check-local-replaces.sh"
FIXTURES="${ROOT}/scripts/testdata/local-replaces"

"${CHECK}" "${FIXTURES}/remote.mod"

for fixture in relative.mod absolute.mod; do
  if "${CHECK}" "${FIXTURES}/${fixture}" >/dev/null 2>&1; then
    echo "expected ${fixture} to reject a local replacement" >&2
    exit 1
  fi
done

echo "local replacement policy fixtures OK"
