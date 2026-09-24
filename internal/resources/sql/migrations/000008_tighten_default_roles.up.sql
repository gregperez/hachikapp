-- Tighten default role permissions to least privilege.
-- admin: full USERS + ROLES + USERS_PRIVILEGES
-- user / guest: no administrative capabilities
-- Idempotent: keyed by role name.

UPDATE public.roles
SET routes = ARRAY['USERS', 'ROLES']::text[],
    permissions = '[
      {"entity":"USERS","create":true,"read":true,"update":true,"delete":true},
      {"entity":"ROLES","create":true,"read":true,"update":true,"delete":true},
      {"entity":"USERS_PRIVILEGES","create":false,"read":false,"update":true,"delete":false}
    ]'::jsonb
WHERE name = 'admin';

UPDATE public.roles
SET routes = ARRAY[]::text[],
    permissions = '[]'::jsonb
WHERE name IN ('user', 'guest');
