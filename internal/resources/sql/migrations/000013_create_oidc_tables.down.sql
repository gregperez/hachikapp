DROP TRIGGER IF EXISTS oidc_transactions_entity_timestamps ON public.oidc_transactions;
DROP INDEX IF EXISTS public.oidc_transactions_grant_expires_at_idx;
DROP INDEX IF EXISTS public.oidc_transactions_expires_at_idx;
DROP TABLE IF EXISTS public.oidc_transactions;

DROP TRIGGER IF EXISTS external_identities_entity_timestamps ON public.external_identities;
DROP INDEX IF EXISTS public.external_identities_user_id_idx;
DROP TABLE IF EXISTS public.external_identities;
