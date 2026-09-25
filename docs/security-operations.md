# Security operations

Operational guide for secrets, administrative bootstrap, and exposure response.
This document never contains real secret values.

## Local configuration

1. Copy the templates (do not commit the local copies):

   ```sh
   cp internal/resources/properties.example.yml internal/resources/properties.yml
   cp .env.example .env
   ```

2. Generate distinct secrets (at least 32 bytes for each JWT secret):

   ```sh
   export DB_PASSWORD="$(openssl rand -base64 24)"
   export JWT_ACCESS_SECRET="$(openssl rand -base64 48)"
   export JWT_REFRESH_SECRET="$(openssl rand -base64 48)"
   ```

3. Export the variables from `.env` (or inject them via your process manager) before starting the process. `initModules` expands `${NAME}` / `$NAME` in YAML via `os.ExpandEnv`.

4. Confirm local files are ignored:

   ```sh
   git check-ignore -v internal/resources/properties.yml .env
   ```

See also the Configuration section in [README.md](../README.md).

## Administrative bootstrap

Migrations must not create a privileged account with a known identity or password.
Create the first administrator once with:

```bash
BOOTSTRAP_ADMIN_NAME='Admin' \
BOOTSTRAP_ADMIN_EMAIL='admin@example.com' \
go run ./cmd/bootstrap-admin
```

Password is read from a non-echoing prompt, or from the short-lived
`BOOTSTRAP_ADMIN_PASSWORD` environment variable (unset immediately after read).
Never pass the password on argv or commit it.

### Required behavior

| Step | Contract |
|---|---|
| Input | Password only via non-echoing prompt (`terminal.ReadPassword`) or a short-lived environment variable that is unset after use. Never argv, never logs, never committed files. |
| Hashing | Generate Argon2id PHC at runtime via authBase. Existing bcrypt hashes remain verify-only during migration. |
| Uniqueness | Fails if any active administrator already exists. |
| Failure | Exit non-zero without modifying data when an admin exists or the password is weak. |
| Atomicity | Persistence runs inside the repository transaction. |
| Output | Prints success without the password or hash. |

### Acceptance tests

Covered by `internal/core/services/user_privilege_test.go`:

- First bootstrap succeeds.
- Second bootstrap is rejected.
- Weak password is rejected.

## Rotating secrets after exposure

Assume every credential or JWT key that was ever committed to this repository is compromised.

Perform the following **outside** the repository (secret manager, cloud IAM, database roles, deployment config). Do not paste old or new values into tickets, chats, or docs.

1. **Inventory** — Identify which environments still use historically versioned DB passwords or JWT secrets (type and location only).
2. **Issue new material** — Create new DB credentials and new access/refresh JWT secrets. Keep access and refresh secrets distinct and long enough (>= 32 bytes).
3. **Deploy new config** — Roll out the new values to the runtime (env / secret store). Prefer overlapping windows where the app can verify with the new secrets after cutover.
4. **Revoke sessions** — Invalidate refresh sessions issued under the old
   secrets. Use family revocation where one family is known and user-wide
   revocation for an affected account. Full-key compromise still requires an
   operator-approved global revoke during a maintenance window; do not expose
   an unauthenticated bulk-revocation endpoint.
5. **Verify** — Confirm health checks, login, and a protected call succeed with the new material.
6. **Invalidate old material** — Rotate or delete the previous DB password and JWT secrets in the provider so they cannot be reused.
7. **Record** — Log date, environment, and operator in an external ops log. Never record secret values.

### History rewrite (coordinated, destructive)

Rewriting Git history to purge old blobs is **out of scope** for routine work. It requires:

- Explicit authorization from repository owners.
- Coordination with every clone, fork, and CI cache.
- Force-push impact on open branches and review links.

Prefer rotation + ignore rules + scanners. Treat history rewrite as a last resort.

## Migration note for `000002` (checksum-preserving)

No live environment inventory was available to prove that `000002` was never applied.
Per the plan stop condition, the historical file is **left unchanged** so existing
`schema_migrations` checksums remain valid.

Forward strategy:

- `000006_revoke_seeded_admin.up.sql` deletes the known seeded administrator by
  natural key (`email = admin@example.com` and `is_admin`) after `000002` runs.
- Fresh installs therefore only retain the privileged seed for the duration of the
  migrate transaction chain until `000006` applies; they must not rely on that account.
- `000006` down is intentionally empty: it must not restore a known password hash.
- Treat the historically versioned seed identity as compromised; rotate or remove it
  in any environment that already applied `000002` even before deploying `000006`.
- A future greenfield-only baseline (squash without the insert) remains optional and
  coordinated; it is not required to keep checksums stable for existing installs.

## Secret scanning

Run locally before opening a PR:

```sh
./scripts/check-secrets.sh
```

This wraps [gitleaks](https://github.com/gitleaks/gitleaks) with:

- `.gitleaks.toml` — rules and minimal allowlist for example templates
- `.gitleaks-baseline.json` — redacted fingerprint of the pre-containment history finding so CI fails only on **new** leaks

Full history purge remains a coordinated, destructive operation (see above). Until then, keep the baseline and rotate any material that was ever versioned.

```sh
# Equivalent direct invocation
gitleaks detect --source . --config .gitleaks.toml \
  --baseline-path .gitleaks-baseline.json --no-banner --redact
```

The same command is intended for CI once the quality baseline (plan 003) wires it into the pipeline.

## Historical token invalidation (pre-v3 → v3)

The initial authBase v3 integration **invalidated all tokens issued by the
pre-v3 implementation**. The later v3 → v4 pagination upgrade does not alter
token claims or require re-login when signing configuration is unchanged.
Clients affected by that historical migration had to log in again. Refresh
tokens are single-use (atomic rotation); reuse of a consumed refresh token
revokes the whole family. Prefer `POST /v1/auth/logout` with the current refresh
token to revoke a family explicitly (idempotent even if already revoked).

Password changes and account deactivation revoke all refresh families for the
user atomically with the account write. Reactivation therefore never restores
an old refresh session; the user must authenticate again.

Expired-session deletion is retention-aware and batch-limited. The lifecycle
component is constructed and tested but remains unregistered until plan 015
owns its startup and shutdown ordering. Operators must choose a forensic
retention window before enabling it in production.

## Password and authentication abuse policy

New passwords contain 15–128 Unicode code points, may contain spaces, and have
no character-class composition rule. The service also rejects contextual terms
(such as the user's name or email components) and entries in an offline
blocklist. `AUTH_PASSWORD_BLOCKLIST` replaces the built-in starter list with a
comma-separated, deployment-owned list. Never send candidate passwords or
their hashes to a remote service from the request path.

authBase creates Argon2id hashes with 19 MiB memory, 2 iterations, parallelism
1, 16-byte salts, and 32-byte keys. bcrypt remains supported only to verify
existing rows; this release does not claim automatic rehash-on-login. Recalibrate
Argon2id on production hardware and review the parameters annually.

Login attempts use independent fixed-window quotas for normalized-account and
origin hashes. Refresh attempts use the origin quota first, then a signed
refresh-family hash only after JWT verification. A successful request clears
the account/family penalty but never the shared origin quota. Configure:

- `AUTH_LOGIN_ACCOUNT_LIMIT` (default `5`)
- `AUTH_LOGIN_ORIGIN_LIMIT` (default `20`)
- `AUTH_REFRESH_FAMILY_LIMIT` (default `10`)
- `AUTH_RATE_LIMIT_WINDOW` (default `15m`)
- `AUTH_RATE_LIMIT_MAX_ENTRIES` (default `10000`)
- `AUTH_TRUSTED_PROXY_CIDRS` (empty by default)

Forwarded headers are ignored unless the immediate peer belongs to an explicitly
trusted CIDR. Configure only proxy networks controlled by the deployment.
The included bounded in-memory limiter is suitable for one replica. Multiple
replicas require an adapter backed by shared storage so every instance enforces
the same counters and windows.

## Adding a new sensitive setting

1. Add the key to `properties.example.yml` and `.env.example` with a placeholder or `${ENV_VAR}`.
2. Document the variable name in this file and the README.
3. Keep the real value only in ignored local files or the secret manager.
4. Extend gitleaks rules or allowlist carefully; keep the allowlist minimal.
5. Never reuse the same secret across development, test, and production.
