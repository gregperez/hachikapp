UPDATE public.roles
SET routes = ARRAY['USERS', 'ROLES', 'AUDIT_EVENTS', 'SETTINGS']::text[],
    permissions = '[
      {"entity":"USERS","create":true,"read":true,"update":true,"delete":true},
      {"entity":"ROLES","create":true,"read":true,"update":true,"delete":true},
      {"entity":"USERS_PRIVILEGES","create":false,"read":false,"update":true,"delete":false},
      {"entity":"AUDIT_EVENTS","create":false,"read":true,"update":false,"delete":false},
      {"entity":"SETTINGS","create":false,"read":true,"update":true,"delete":false}
    ]'::jsonb
WHERE name = 'admin';
