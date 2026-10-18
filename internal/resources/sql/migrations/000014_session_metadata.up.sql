-- Minimal display metadata for self-service session management.
-- Never stores JWTs or precise geolocation.

ALTER TABLE public.refresh_sessions
    ADD COLUMN IF NOT EXISTS client_label varchar(120) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS ip_prefix varchar(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_seen_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS family_started_at timestamptz NOT NULL DEFAULT now();

UPDATE public.refresh_sessions
SET family_started_at = issued_at,
    last_seen_at = COALESCE(consumed_at, issued_at);

CREATE INDEX IF NOT EXISTS refresh_sessions_user_id_expires_at_idx
    ON public.refresh_sessions (user_id, expires_at);

CREATE INDEX IF NOT EXISTS refresh_sessions_user_active_cursor_idx
    ON public.refresh_sessions (user_id, last_seen_at DESC, family_id DESC)
    WHERE revoked_at IS NULL AND consumed_at IS NULL;
