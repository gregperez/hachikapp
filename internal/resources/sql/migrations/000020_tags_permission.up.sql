-- Admin role gets the TAGS route and permission (create and read tag batches).

UPDATE public.roles
SET routes = ARRAY['USERS', 'ROLES', 'AUDIT_EVENTS', 'SETTINGS', 'TAGS']::text[],
    permissions = '[
      {"entity":"USERS","create":true,"read":true,"update":true,"delete":true},
      {"entity":"ROLES","create":true,"read":true,"update":true,"delete":true},
      {"entity":"USERS_PRIVILEGES","create":false,"read":false,"update":true,"delete":false},
      {"entity":"AUDIT_EVENTS","create":false,"read":true,"update":false,"delete":false},
      {"entity":"SETTINGS","create":false,"read":true,"update":true,"delete":false},
      {"entity":"TAGS","create":true,"read":true,"update":false,"delete":false}
    ]'::jsonb
WHERE name = 'admin';
