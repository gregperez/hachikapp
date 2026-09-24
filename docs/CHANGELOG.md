# Changelog

## Unreleased

### Breaking

- HTTP error responses now use `application/problem+json` with stable `code`
  fields instead of the previous `ResponseDto` envelope on 4xx/5xx
  (plan 008 / O-023). Successful 2xx responses still use `ResponseDto`.

### Security / reliability

- Privileged boolean request fields (`active`, `isAdmin`) require an explicit
  value; omitted fields return 422 instead of defaulting to `false` (O-024).
- Unique conflicts map to HTTP 409 (O-025).
- Logout returns 5xx when refresh-family revocation fails operationally (O-004).
- Production validation applies TLS/placeholder rules to `DATABASE_URL` (O-012).
- Config parse errors for ints/durations/bools abort startup (O-013).
- Go minimum raised to 1.26.6 (O-029).
- Depend on published `authBase/v3@v3.0.1` and `initModules/v2@v2.0.0` without
  a local `replace` (O-030). Prefer `v3.0.1` over `v3.0.0` because the Go
  module proxy froze an earlier `v3.0.0` tree.
- Process logging migrated from Logrus to `log/slog` (O-027 / plan 011).
