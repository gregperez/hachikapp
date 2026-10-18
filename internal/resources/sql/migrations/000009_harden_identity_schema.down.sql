-- Reverse objects added by 000009. Does not recreate sanitized null/duplicate rows.

ALTER TABLE public.roles
    DROP CONSTRAINT IF EXISTS roles_name_key;

DROP INDEX IF EXISTS public.users_roles_role_id_idx;
DROP INDEX IF EXISTS public.users_roles_user_id_idx;

ALTER TABLE public.users_roles
    DROP CONSTRAINT IF EXISTS users_roles_user_id_role_id_key;

ALTER TABLE public.users_roles
    ALTER COLUMN user_id DROP NOT NULL,
    ALTER COLUMN role_id DROP NOT NULL;
