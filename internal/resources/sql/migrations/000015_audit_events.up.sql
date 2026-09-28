-- Append-only administrative audit trail. Never stores JWTs, passwords,
-- emails, cookies, bodies, or prompts.

CREATE TABLE public.audit_events (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    category varchar(32) NOT NULL,
    action varchar(64) NOT NULL,
    outcome varchar(16) NOT NULL,
    actor_id varchar(64) NOT NULL,
    subject_type varchar(32) NOT NULL DEFAULT '',
    subject_id varchar(64) NOT NULL DEFAULT '',
    request_id varchar(64) NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT audit_events_category_chk CHECK (category IN ('security', 'privilege')),
    CONSTRAINT audit_events_outcome_chk CHECK (outcome IN ('success', 'failure'))
);

CREATE INDEX audit_events_cursor_idx
    ON public.audit_events (occurred_at DESC, id DESC);

CREATE INDEX audit_events_actor_idx
    ON public.audit_events (actor_id, occurred_at DESC);

CREATE INDEX audit_events_subject_idx
    ON public.audit_events (subject_id, occurred_at DESC);

CREATE INDEX audit_events_action_idx
    ON public.audit_events (action, occurred_at DESC);

CREATE OR REPLACE FUNCTION public.reject_audit_event_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only';
END;
$$;

CREATE TRIGGER audit_events_no_update
    BEFORE UPDATE ON public.audit_events
    FOR EACH ROW
EXECUTE FUNCTION public.reject_audit_event_update();

-- admin: read-only access to the consultable trail (least privilege).
UPDATE public.roles
SET routes = ARRAY['USERS', 'ROLES', 'AUDIT_EVENTS']::text[],
    permissions = '[
      {"entity":"USERS","create":true,"read":true,"update":true,"delete":true},
      {"entity":"ROLES","create":true,"read":true,"update":true,"delete":true},
      {"entity":"USERS_PRIVILEGES","create":false,"read":false,"update":true,"delete":false},
      {"entity":"AUDIT_EVENTS","create":false,"read":true,"update":false,"delete":false}
    ]'::jsonb
WHERE name = 'admin';
