-- Singleton typed application settings. Never stores JWT, DSN, CORS, or secrets.

CREATE TABLE public.app_settings (
    id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    display_name varchar(80) NOT NULL,
    logo_url varchar(2048) NOT NULL DEFAULT '',
    default_locale varchar(16) NOT NULL,
    version bigint NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by varchar(64) NOT NULL DEFAULT 'system',
    CONSTRAINT app_settings_display_name_chk CHECK (char_length(btrim(display_name)) BETWEEN 1 AND 80),
    CONSTRAINT app_settings_locale_chk CHECK (default_locale IN ('es', 'en', 'es-CL', 'en-US'))
);

INSERT INTO public.app_settings (id, display_name, logo_url, default_locale, version, updated_by)
VALUES (1, 'Base Console', '', 'es', 1, 'system');

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
