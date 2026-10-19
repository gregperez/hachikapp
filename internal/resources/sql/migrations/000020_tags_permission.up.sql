-- Admin role gets the TAGS route and permission (create and read tag batches).
-- Additive and idempotent: other routes and permissions are left untouched.

UPDATE public.roles
SET routes = COALESCE(routes, ARRAY[]::text[]) || ARRAY['TAGS']::text[]
WHERE name = 'admin'
  AND NOT (COALESCE(routes, ARRAY[]::text[]) @> ARRAY['TAGS']::text[]);

UPDATE public.roles
SET permissions = COALESCE(permissions, '[]'::jsonb)
    || '[{"entity":"TAGS","create":true,"read":true,"update":false,"delete":false}]'::jsonb
WHERE name = 'admin'
  AND NOT (COALESCE(permissions, '[]'::jsonb) @> '[{"entity":"TAGS"}]'::jsonb);
