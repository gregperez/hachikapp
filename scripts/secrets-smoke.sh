#!/usr/bin/env bash
# Smoke checks for O-001 / O-003: local env files ignored; gitleaks config
# must not allowlist entire docs/ or plans/ trees.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "check-ignore: real env files must be ignored"
for f in .env .env.staging .env.production .env.local; do
  if ! git check-ignore -q "$f"; then
    echo "expected $f to be ignored" >&2
    exit 1
  fi
done

echo "check-ignore: .env.example must remain trackable"
if git check-ignore -q .env.example; then
  echo ".env.example must not be ignored" >&2
  exit 1
fi

echo "gitleaks allowlist: no broad docs/ or plans/ path exclusions"
if grep -E "plans/|docs/security-operations" "${ROOT}/.gitleaks.toml" >/dev/null 2>&1; then
  echo ".gitleaks.toml still allowlists documentation trees" >&2
  exit 1
fi

GITLEAKS_BIN="${ROOT}/.tmp/bin/gitleaks"
if [[ ! -x "${GITLEAKS_BIN}" ]] && command -v gitleaks >/dev/null 2>&1; then
  GITLEAKS_BIN="$(command -v gitleaks)"
fi
if [[ ! -x "${GITLEAKS_BIN}" ]]; then
  echo "gitleaks binary is required for the negative fixture check" >&2
  exit 1
fi

echo "gitleaks: a private-key fixture without the historical sentinel must still fail"
NEG_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gitleaks-neg.XXXXXX")"
cleanup_neg() { rm -rf "${NEG_DIR}"; }
trap cleanup_neg EXIT
cat > "${NEG_DIR}/leak.pem" <<'EOF'
-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
-----END PRIVATE KEY-----
EOF
if "${GITLEAKS_BIN}" detect --no-git --source "${NEG_DIR}" --config "${ROOT}/.gitleaks.toml" --no-banner --redact >/dev/null 2>&1; then
  echo "expected gitleaks to detect an independent private-key fixture" >&2
  exit 1
fi

echo "secrets-smoke OK"
