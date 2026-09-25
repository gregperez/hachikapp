DROP INDEX IF EXISTS public.roles_created_at_id_idx;
DROP INDEX IF EXISTS public.users_created_at_id_idx;

DROP TRIGGER IF EXISTS users_roles_entity_timestamps ON public.users_roles;
DROP TRIGGER IF EXISTS roles_entity_timestamps ON public.roles;
DROP TRIGGER IF EXISTS users_entity_timestamps ON public.users;

ALTER TABLE public.users_roles
    ALTER COLUMN updated_at DROP NOT NULL,
    ALTER COLUMN created_at DROP NOT NULL;

ALTER TABLE public.roles
    ALTER COLUMN updated_at DROP NOT NULL,
    ALTER COLUMN created_at DROP NOT NULL;

ALTER TABLE public.users
    ALTER COLUMN updated_at DROP NOT NULL,
    ALTER COLUMN created_at DROP NOT NULL;

DROP FUNCTION IF EXISTS public.set_entity_timestamps();
