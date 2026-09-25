CREATE OR REPLACE FUNCTION public.set_entity_timestamps()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        NEW.created_at := COALESCE(NEW.created_at, statement_timestamp());
        NEW.updated_at := COALESCE(NEW.updated_at, NEW.created_at);
    ELSE
        NEW.created_at := OLD.created_at;
        NEW.updated_at := statement_timestamp();
    END IF;

    RETURN NEW;
END;
$$;

UPDATE public.users
SET created_at = COALESCE(created_at, statement_timestamp());

UPDATE public.users
SET updated_at = COALESCE(updated_at, created_at);

UPDATE public.roles
SET created_at = COALESCE(created_at, statement_timestamp());

UPDATE public.roles
SET updated_at = COALESCE(updated_at, created_at);

UPDATE public.users_roles
SET created_at = COALESCE(created_at, statement_timestamp());

UPDATE public.users_roles
SET updated_at = COALESCE(updated_at, created_at);

ALTER TABLE public.users
    ALTER COLUMN created_at SET NOT NULL,
    ALTER COLUMN updated_at SET NOT NULL;

ALTER TABLE public.roles
    ALTER COLUMN created_at SET NOT NULL,
    ALTER COLUMN updated_at SET NOT NULL;

ALTER TABLE public.users_roles
    ALTER COLUMN created_at SET NOT NULL,
    ALTER COLUMN updated_at SET NOT NULL;

CREATE TRIGGER users_entity_timestamps
    BEFORE INSERT OR UPDATE ON public.users
    FOR EACH ROW
EXECUTE FUNCTION public.set_entity_timestamps();

CREATE TRIGGER roles_entity_timestamps
    BEFORE INSERT OR UPDATE ON public.roles
    FOR EACH ROW
EXECUTE FUNCTION public.set_entity_timestamps();

CREATE TRIGGER users_roles_entity_timestamps
    BEFORE INSERT OR UPDATE ON public.users_roles
    FOR EACH ROW
EXECUTE FUNCTION public.set_entity_timestamps();

CREATE INDEX users_created_at_id_idx
    ON public.users (created_at DESC, id DESC);

CREATE INDEX roles_created_at_id_idx
    ON public.roles (created_at DESC, id DESC);
