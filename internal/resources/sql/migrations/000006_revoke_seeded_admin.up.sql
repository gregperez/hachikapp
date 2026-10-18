-- Revoke the historically seeded privileged account from 000002.
-- 000002 is left unchanged so existing schema_migrations checksums remain valid.
-- Fresh installs still execute 000002, then this migration removes the known seed.
-- Do not re-introduce password hashes here. Bootstrap: docs/security-operations.md

DELETE FROM users
WHERE email = 'admin@example.com'
  AND is_admin IS TRUE;
