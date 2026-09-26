# Changelog

## Unreleased

### Breaking

- HTTP error responses now use `application/problem+json` with stable `code`
  fields instead of the previous `ResponseDto` envelope on 4xx/5xx
  (plan 008 / O-023). Successful 2xx responses still use `ResponseDto`.
- User and role listings now use forward-only cursor pagination ordered by
  `(created_at, id)` descending; page numbers and exact totals were removed.

### Security / reliability

- Optional Google Sign-In uses OpenID Connect Authorization Code + PKCE.
  Identities bind by `(issuer, subject)` after an explicit authenticated link.
  The callback never places local or Google tokens in the redirect URL.

- Privileged boolean request fields (`active`, `isAdmin`) require an explicit
  value; omitted fields return 422 instead of defaulting to `false` (O-024).
- Unique conflicts map to HTTP 409 (O-025).
- Logout returns 5xx when refresh-family revocation fails operationally (O-004).
- Production validation applies TLS/placeholder rules to `DATABASE_URL` (O-012).
- Config parse errors for ints/durations/bools abort startup (O-013).
- Go minimum raised to 1.26.6 (O-029).
- Depend on `authBase/v4@v4.2.0` for typed keyset pagination, additive
  session lifecycle contracts, equalized login verification, and Argon2id
  hashing, and on
  `initModules/v2@v2.0.0`.
- Password changes and account deactivation now revoke all refresh families in
  the same PostgreSQL transaction as the account mutation.
- Added bounded, retention-aware expired-session cleanup prepared for lifecycle
  registration in plan 015.
- New passwords use a length-first Unicode policy (15–128 code points), an
  injected offline blocklist, and contextual-term rejection.
- Login and refresh endpoints enforce bounded account, trusted-origin, and
  verified refresh-family quotas with RFC 9457 `429` responses.
- New credential hashes use Argon2id through authBase v4.2; bcrypt remains
  verify-only for existing rows.
- Process composition uses an isolated `Application` with `initModules.NewApp`
  lifecycles (DB, translations, session purge, HTTP). Production paths no
  longer install a global logger or database singleton.
- Process logging migrated from Logrus to `log/slog` (O-027 / plan 011).
