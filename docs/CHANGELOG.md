# Changelog

## Unreleased

### Breaking

- HTTP error responses now use `application/problem+json` with stable `code`
  fields instead of the previous `ResponseDto` envelope on 4xx/5xx
  (plan 008 / O-023). Successful 2xx responses still use `ResponseDto`.
- User and role listings now use forward-only cursor pagination ordered by
  `(created_at, id)` descending; page numbers and exact totals were removed.

### Security / reliability

- Privileged boolean request fields (`active`, `isAdmin`) require an explicit
  value; omitted fields return 422 instead of defaulting to `false` (O-024).
- Unique conflicts map to HTTP 409 (O-025).
- Logout returns 5xx when refresh-family revocation fails operationally (O-004).
- Production validation applies TLS/placeholder rules to `DATABASE_URL` (O-012).
- Config parse errors for ints/durations/bools abort startup (O-013).
- Go minimum raised to 1.26.6 (O-029).
- Depend on `authBase/v4@v4.1.0` for typed keyset pagination and additive
  session lifecycle contracts, and on
  `initModules/v2@v2.0.0`.
- Password changes and account deactivation now revoke all refresh families in
  the same PostgreSQL transaction as the account mutation.
- Added bounded, retention-aware expired-session cleanup prepared for lifecycle
  registration in plan 015.
- Process logging migrated from Logrus to `log/slog` (O-027 / plan 011).
