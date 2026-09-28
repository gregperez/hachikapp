DROP INDEX IF EXISTS public.refresh_sessions_user_active_cursor_idx;
DROP INDEX IF EXISTS public.refresh_sessions_user_id_expires_at_idx;

ALTER TABLE public.refresh_sessions
    DROP COLUMN IF EXISTS family_started_at,
    DROP COLUMN IF EXISTS last_seen_at,
    DROP COLUMN IF EXISTS ip_prefix,
    DROP COLUMN IF EXISTS client_label;
