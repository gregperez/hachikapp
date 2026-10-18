UPDATE public.roles
SET routes = ARRAY['USERS', 'ROLES']::text[],
    permissions = '[
      {"entity":"USERS","create":true,"read":true,"update":true,"delete":true},
      {"entity":"ROLES","create":true,"read":true,"update":true,"delete":true},
      {"entity":"USERS_PRIVILEGES","create":false,"read":false,"update":true,"delete":false}
    ]'::jsonb
WHERE name = 'admin';

DROP TRIGGER IF EXISTS audit_events_no_update ON public.audit_events;
DROP FUNCTION IF EXISTS public.reject_audit_event_update();
DROP INDEX IF EXISTS public.audit_events_action_idx;
DROP INDEX IF EXISTS public.audit_events_subject_idx;
DROP INDEX IF EXISTS public.audit_events_actor_idx;
DROP INDEX IF EXISTS public.audit_events_cursor_idx;
DROP TABLE IF EXISTS public.audit_events;
