-- Remove exactly the TAGS route and the TAGS permission from the admin role.
-- Idempotent: other routes and permissions are left untouched.

UPDATE public.roles
SET routes = array_remove(routes, 'TAGS')
WHERE name = 'admin'
  AND routes @> ARRAY['TAGS']::text[];

UPDATE public.roles
SET permissions = COALESCE(
      (SELECT jsonb_agg(p ORDER BY ord)
       FROM jsonb_array_elements(permissions) WITH ORDINALITY AS e(p, ord)
       WHERE NOT (p @> '{"entity":"TAGS"}'::jsonb)),
      '[]'::jsonb)
WHERE name = 'admin'
  AND permissions @> '[{"entity":"TAGS"}]'::jsonb;
