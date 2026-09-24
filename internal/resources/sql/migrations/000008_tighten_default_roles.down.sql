-- Intentionally does NOT restore the pre-004 overly permissive role grants.
-- Rolling back privilege hardening must not re-open escalation paths in any environment.
-- To revert locally for experiments, recreate the database from migrations or seed explicitly.
SELECT 1;
