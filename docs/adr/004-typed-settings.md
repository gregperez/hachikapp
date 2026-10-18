# ADR 004 — Typed application settings

## Status

Accepted (plan 006)

## Context

The console needs visible branding and localization without becoming an editor
for infrastructure. Secrets, JWT material, DSNs, CORS, and timeouts stay in
deployment configuration.

## Decision

1. **Three layers.** Personal preferences (theme, locale, timezone) belong to
   the user and are not stored with administrative settings. Application
   settings are a single typed row: `displayName`, `logoUrl`, and
   `defaultLocale`. Operational secrets never appear in this API or UI.
2. **Schema.** `app_settings` is a singleton (`id = 1`) with typed columns,
   defaults, and a monotonic `version`. The HTTP contract is a closed struct,
   not a key/value map.
3. **Public vs administrative.** `GET /v1/settings/public` returns only the
   three branding fields and needs no authentication. `GET /v1/settings`
   requires `SETTINGS.read` and also returns `version`, `updatedAt`, and
   `updatedBy`. `PUT /v1/settings` requires `SETTINGS.update`, an expected
   version, and the allowlisted fields.
4. **Validation.** Unknown or protected keys (`jwt*`, `dsn`, `database*`,
   `cors`, `apiKey`, tokens, passwords) yield 422. `logoUrl` is empty, `https`,
   or a same-origin path starting with `/`. Locales are a closed set: `es`,
   `en`, `es-CL`, `en-US`.
5. **Concurrency.** Updates compare `version` (body, and `If-Match` when
   present). A mismatch is 409. Successful writes increment `version`.
6. **Cache.** Public responses send `ETag` (the version) and
   `Cache-Control: public, max-age=60`. Administrative GET is `private, no-store`.
7. **Audit.** Mutations emit `update_settings` with metadata `fields` listing
   changed names only. Values are never persisted in the audit trail.

## Consequences

- Adding a key requires schema, defaults, validation, OpenAPI, UI, and tests.
- Personal preferences may later use `/v1/users/me/preferences`; they must not
  mix with `SETTINGS`.
- Feature flags, file uploads, and tenant-scoped settings need another ADR.
