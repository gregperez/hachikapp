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
- A chip number is unique per account (migration `000018`), so nobody can
  squat another owner's chip or probe whether a chip is registered.
  Updates keep the stored visibility of keys the request omits.
- Pet photos (JPEG/PNG up to 1 MiB and 2048 px per side, type sniffed from
  bytes) are re-encoded on the server: EXIF orientation is applied and every
  metadata block (EXIF/GPS, comments, ICC color profiles) is dropped so a
  photo cannot reveal the owner's location. JPEG quality steps down from 88
  until the result fits in 1 MiB. They are stored in PostgreSQL behind `PetPhotoPort`, ready
  to move to object storage.
- Pet tags ("Ficha pública", part 1). Migrations `000019_pet_tags` (tags,
  batches, scans and finder messages) and `000020_tags_permission`, which
  gives the `admin` role the new `TAGS` route and permission (create, read).
  - Public routes without login: `GET /v1/public/tags/{code}` and its
    `/photo`, `POST /v1/public/tags/{code}/scans {visitorId}`, and
    `POST /v1/public/scans/{scanId}/messages` and `/contact` with the
    `X-Scan-Token` header. The profile only carries the fields the owner made
    public (first names only, `ageMonths` instead of the birth date, comuna
    never with digits). Address, chip number, surnames and phone numbers are
    never returned; `/contact` returns only the main phone and only when the
    contact button is visible (else 404). Unknown tags, scans and invalid
    tokens are the same 404. Every public response is `Cache-Control:
    no-store` and `X-Robots-Tag: noindex`. A filled `website` honeypot is
    answered 202 and dropped.
  - Per-IP quotas (429 with `Retry-After`): profile and photo 60/min, scans
    20/h, messages 5/h, contact 3/h. The limiter is in memory; with more than
    one instance it must move to Redis.
  - `AUTH_TRUSTED_PROXY_CIDRS` must include the frontend (Next) server's
    address: public tag calls and login arrive from it, and it forwards the
    visitor's IP in `X-Forwarded-For` (frontend `CLIENT_IP_HEADER`). With the
    list empty, every visitor shares one quota; production now logs a startup
    warning in that case.
  - Owner routes: `GET/POST /v1/pets/{id}/tags` (POST creates a digital tag),
    `POST /v1/pets/{id}/tags/{tagId}/disable`, `POST /v1/tags/{code}/activate
    {petId}` (409 when the tag is already linked or the pet has 5 active tags)
    and `GET /v1/pets/{id}/scans` (keyset paginated, with finder messages).
  - Admin routes (`TAGS`): `GET/POST /v1/admin/tag-batches` (1 to 1000 codes)
    and `GET /v1/admin/tag-batches/{id}/codes.csv` (`code,url` with
    `PUBLIC_BASE_URL/t/<code>`).
  - New configuration: `PUBLIC_BASE_URL` (public site origin for tag links)
    and `TAG_VISITOR_PEPPER` (keys visitor, scan-token and rate-limit hashes).
    Production requires an `https://` base URL and a pepper of at least 32
    characters other than the development default.

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
