#!/usr/bin/env bash
# Scan git history + working tree for secrets via gitleaks.
# Uses the versioned config and baseline so known historical findings
# (pre-containment) do not fail the check; new leaks still fail.
# Exit non-zero on new findings. Output is redacted.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CONFIG="${ROOT}/.gitleaks.toml"
BASELINE="${ROOT}/.gitleaks-baseline.json"

ARGS=(
  detect
  --source "${ROOT}"
  --config "${CONFIG}"
  --baseline-path "${BASELINE}"
  --no-banner
  --redact
  --verbose
)

run_gitleaks() {
  local bin="$1"
  shift
  exec "$bin" "$@"
}

if [[ -x "${ROOT}/.tmp/bin/gitleaks" ]]; then
  run_gitleaks "${ROOT}/.tmp/bin/gitleaks" "${ARGS[@]}"
fi

if command -v gitleaks >/dev/null 2>&1; then
  run_gitleaks gitleaks "${ARGS[@]}"
fi

if command -v docker >/dev/null 2>&1; then
  echo "gitleaks binary not found; running via Docker image zricethezav/gitleaks:latest" >&2
  exec docker run --rm \
    -v "${ROOT}:/repo" \
    -w /repo \
    zricethezav/gitleaks:latest \
    detect \
    --source /repo \
    --config /repo/.gitleaks.toml \
    --baseline-path /repo/.gitleaks-baseline.json \
    --no-banner \
    --redact \
    --verbose
fi

cat >&2 <<'EOF'
gitleaks is not installed and Docker is unavailable.
Install one of:
  - brew install gitleaks
  - https://github.com/gitleaks/gitleaks#installing
  - Docker, then re-run ./scripts/check-secrets.sh
EOF
exit 127
