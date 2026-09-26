-- Federated identities and single-use OIDC transactions (plan 016).
-- State, PKCE verifier, and grants are stored hashed or encrypted; never log them.

CREATE TABLE IF NOT EXISTS public.external_identities
(
    id            uuid        DEFAULT uuid_generate_v4() NOT NULL PRIMARY KEY,
    provider      text                                   NOT NULL,
    issuer        text                                   NOT NULL,
    subject       text                                   NOT NULL,
    user_id       uuid                                   NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    email_at_link text,
    last_login_at timestamptz,
    created_at    timestamptz DEFAULT statement_timestamp() NOT NULL,
    updated_at    timestamptz DEFAULT statement_timestamp() NOT NULL,
    CONSTRAINT external_identities_issuer_subject_key UNIQUE (issuer, subject),
    CONSTRAINT external_identities_user_provider_key UNIQUE (user_id, provider)
);

CREATE INDEX IF NOT EXISTS external_identities_user_id_idx
    ON public.external_identities (user_id);

CREATE TRIGGER external_identities_entity_timestamps
    BEFORE INSERT OR UPDATE ON public.external_identities
    FOR EACH ROW
EXECUTE FUNCTION public.set_entity_timestamps();

CREATE TABLE IF NOT EXISTS public.oidc_transactions
(
    id                      uuid        DEFAULT uuid_generate_v4() NOT NULL PRIMARY KEY,
    provider                text                                   NOT NULL,
    state_hash              bytea                                  NOT NULL,
    nonce                   bytea                                  NOT NULL,
    pkce_verifier_ciphertext bytea                                 NOT NULL,
    intent                  text                                   NOT NULL,
    actor_user_id           uuid REFERENCES public.users (id) ON DELETE CASCADE,
    return_to               text                                   NOT NULL,
    expires_at              timestamptz                            NOT NULL,
    consumed_at             timestamptz,
    grant_hash              bytea,
    grant_expires_at        timestamptz,
    grant_consumed_at       timestamptz,
    resolved_user_id        uuid REFERENCES public.users (id) ON DELETE CASCADE,
    created_at              timestamptz DEFAULT statement_timestamp() NOT NULL,
    updated_at              timestamptz DEFAULT statement_timestamp() NOT NULL,
    CONSTRAINT oidc_transactions_state_hash_key UNIQUE (state_hash),
    CONSTRAINT oidc_transactions_grant_hash_key UNIQUE (grant_hash),
    CONSTRAINT oidc_transactions_intent_check CHECK (intent IN ('login', 'link')),
    CONSTRAINT oidc_transactions_link_actor_check CHECK (
        (intent = 'login' AND actor_user_id IS NULL)
            OR (intent = 'link' AND actor_user_id IS NOT NULL)
        )
);

CREATE INDEX IF NOT EXISTS oidc_transactions_expires_at_idx
    ON public.oidc_transactions (expires_at);

CREATE INDEX IF NOT EXISTS oidc_transactions_grant_expires_at_idx
    ON public.oidc_transactions (grant_expires_at);

CREATE TRIGGER oidc_transactions_entity_timestamps
    BEFORE INSERT OR UPDATE ON public.oidc_transactions
    FOR EACH ROW
EXECUTE FUNCTION public.set_entity_timestamps();
