-- Restore pre-004 overly permissive defaults (only for local rollback / tests).
-- WARNING: do not apply in production to re-open privilege escalation.

UPDATE public.roles
SET routes = ARRAY['USERS', 'ROLES']::text[],
    permissions = '[
      {"entity":"USERS","create":true,"read":true,"update":true,"delete":true},
      {"entity":"ROLES","create":true,"read":true,"update":true,"delete":true}
    ]'::jsonb
WHERE name IN ('admin', 'user', 'guest');
