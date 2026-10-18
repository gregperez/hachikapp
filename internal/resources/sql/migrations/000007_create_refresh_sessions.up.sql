-- Refresh sessions for authBase v3 atomic rotation.
-- Stores SHA-256 hashes only; never plaintext refresh JWTs.

CREATE TABLE IF NOT EXISTS public.refresh_sessions
(
    token_hash  bytea       NOT NULL,
    token_id    text        NOT NULL,
    family_id   text        NOT NULL,
    user_id     uuid        NOT NULL REFERENCES public.users (id),
    issued_at   timestamptz NOT NULL,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz NULL,
    revoked_at  timestamptz NULL,
    CONSTRAINT refresh_sessions_token_hash_key UNIQUE (token_hash),
    CONSTRAINT refresh_sessions_token_id_key UNIQUE (token_id)
);

CREATE INDEX IF NOT EXISTS refresh_sessions_family_id_idx
    ON public.refresh_sessions (family_id);

CREATE INDEX IF NOT EXISTS refresh_sessions_user_id_idx
    ON public.refresh_sessions (user_id);

CREATE INDEX IF NOT EXISTS refresh_sessions_expires_at_idx
    ON public.refresh_sessions (expires_at);
