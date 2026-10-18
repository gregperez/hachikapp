# Changelog

## Unreleased

### Added

- Pet records ("Ficha Mascota"): `GET/POST /v1/pets`, `GET/PUT/DELETE
  /v1/pets/{id}` and `GET/PUT/DELETE /v1/pets/{id}/photo` for the
  authenticated account. Records store identification, health conditions,
  the chip-registered owner and an optional second owner as contact data,
  and per-field public visibility (`hidden` / `lost_only` / `always`) with
  defaults from the brand manual. Exact address, chip number and phone
  numbers are never public fields. Migration `000017_pets`.
- Pet photos (JPEG/PNG up to 1 MiB and 4096 px per side, type sniffed from
  bytes) are re-encoded on the server: EXIF orientation is applied and every
  metadata block (EXIF/GPS, comments) is dropped so a photo cannot reveal the
  owner's location. They are stored in PostgreSQL behind `PetPhotoPort`, ready
  to move to object storage.

### Breaking

- HTTP error responses now use `application/problem+json` with stable `code`
  fields instead of the previous `ResponseDto` envelope on 4xx/5xx
  (plan 008 / O-023). Successful 2xx responses still use `ResponseDto`.
- User and role listings now use forward-only cursor pagination ordered by
  `(created_at, id)` descending; page numbers and exact totals were removed.

### Security / reliability

- Optional Ed25519 JWT signing with local `kid` lookup, HMAC overlap, and a
  public access-only JWKS endpoint. Typed security events go to slog without
  tokens or secrets.

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
- Depend on `authBase/v4@v4.3.1` for isolated access/refresh key material,
  JWT key agility, typed security events, session lifecycle contracts,
  equalized login verification, and Argon2id hashing, and on
  `initModules/v2@v2.0.1`.
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
