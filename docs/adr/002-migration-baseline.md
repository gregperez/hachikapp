# ADR 002 — Migration baseline and historical checksums

## Status

Accepted (plan 007)

## Context

Migrations `000001`–`000005` shipped with empty `.down.sql` files and
`OWNER TO postgres` clauses. golang-migrate checksums source files; editing an
already-applied migration in a long-lived database requires an explicit
`force` / re-baseline.

This repository is the product template and CI uses an ephemeral database with
full `up → down -all → up` verification, so correcting the historical files is
preferable to leaving irreversible no-op downs and a hard dependency on a role
named `postgres`.

## Decision

1. **Amend** `000001`–`000005` to remove `OWNER TO postgres` and to provide real
   downs (drop tables / delete seed rows by natural key).
2. **Do not** introduce new `OWNER TO` clauses. Objects are owned by the role
   that runs the migrator.
3. **Add** forward migration `000009_harden_identity_schema` for NOT NULL,
   unique constraints, and indexes on identity relations.
4. **Remove** `internal/resources/init.sql` from the runtime path (archived under
   `docs/archive/` as non-executable reference).
5. **Revoke** the historical privileged seed with forward migration `000006`;
   its down migration never restores known credentials.
6. **Add** `000007_create_refresh_sessions` for hashed, rotating refresh-token
   families and `000008_tighten_default_roles` for catalog hardening.
7. **Manage** entity timestamps in PostgreSQL with
   `000012_manage_entity_timestamps`; `(created_at, id)` is the stable keyset
   used by user and role listings.

## Consequences for existing deployments

If a durable environment already applied the previous checksums of
`000001`–`000005`:

1. Inventory whether downs were ever relied upon (they were no-ops).
2. Prefer a maintenance window that rebuilds from migrations on a fresh volume,
   **or** run `migrate force <version>` only after comparing the intended SQL
   and accepting checksum drift.
3. Then apply all forward migrations from `000006` through `000012` as usual.

Fresh installs and CI need no special handling.

## Role name policy

`roles.name` uniqueness is **case-sensitive** as stored. Seed names are
lowercase (`admin`, `user`, `guest`). Application code must not introduce
case-variant duplicates.

## FK policy

`users_roles` retains `ON DELETE CASCADE` to `users` and `roles`. Deleting a
user or role removes junction rows.
