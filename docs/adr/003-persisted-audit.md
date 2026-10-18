# ADR 003 — Persisted administrative audit

## Status

Accepted (plan 005)

## Context

Authentication and privilege changes already emit structured logs, but operators
cannot query who did what, on which resource, and when. A consultable trail must
not become a store of secrets, emails, request bodies, or AI prompts.

## Decision

1. **Taxonomy.** Two categories: `security` (auth/session/OIDC) and `privilege`
   (administrative mutations). Actions are a closed allowlist. Outcomes are
   `success` or `failure`.
2. **Allowlisted fields.** Persisted and returned records contain only: event ID
   (UUID), `occurredAt`, `category`, `action`, `outcome`, `actorId`,
   `subjectType`, `subjectId`, `requestId`, and sanitized `metadata`. Metadata
   keys are limited to `provider`, `roleIds`, `isAdmin`, `familyId`, and
   `tokenId` (identifiers, never JWTs).
3. **Actor.** Empty actor IDs are stored as `system` (key rotation, anonymous
   login failures, token rejects without a subject).
4. **Ordering.** Listings use keyset `(occurred_at DESC, id DESC)` so equal
   timestamps remain stable.
5. **Append-only.** The application never UPDATEs `audit_events`. A BEFORE
   UPDATE trigger rejects mutations. DELETE is allowed only for retention purge.
6. **Sink failure.** Persistence is a secondary observer. A down or slow store
   must not fail login, refresh, OIDC, or privilege mutations. Failures emit an
   error log (`audit persist failed`) without the original payload. There is no
   atomicity requirement between the business write and the audit row.
7. **Retention.** Default 90 days. A background lifecycle deletes older rows in
   bounded batches. Changing the window requires this ADR plus a privacy review.
8. **Authorization.** `GET /v1/audit-events` requires `AUDIT_EVENTS.read`.
   `is_admin` continues to bypass PoliciesGuard. User/guest seeds stay empty.

## Consequences

- Operators query a sanitized trail from the console.
- Exporting to SIEM or shortening retention is a later ADR.
- PrivilegeAuditor and SecurityEventSink fan in to one recorder so each
  occurrence is persisted once.
