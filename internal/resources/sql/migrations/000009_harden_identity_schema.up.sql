-- Harden identity schema (plan 007).
--
-- Invariants:
--   * roles.name is UNIQUE (case-sensitive as stored; seeds use lowercase).
--   * users_roles.(user_id, role_id) are NOT NULL, UNIQUE, FK CASCADE (existing).
--   * Lookup indexes on users_roles by user_id and role_id.
--
-- Sanitization is deterministic and only removes null/duplicate junction rows
-- and duplicate role names (keeps earliest by created_at, then id).

-- --- users_roles: drop null FKs ---
DELETE FROM public.users_roles
WHERE user_id IS NULL
   OR role_id IS NULL;

-- --- users_roles: drop duplicate pairs (keep lowest id) ---
DELETE FROM public.users_roles a
WHERE a.id IN (
    SELECT id
    FROM (
             SELECT id,
                    ROW_NUMBER() OVER (PARTITION BY user_id, role_id ORDER BY id) AS rn
             FROM public.users_roles
         ) d
    WHERE d.rn > 1
);

-- --- roles: collapse duplicate names ---
CREATE TEMP TABLE _007_role_keep AS
SELECT DISTINCT ON (name) id AS keep_id, name
FROM public.roles
ORDER BY name, created_at NULLS LAST, id;

CREATE TEMP TABLE _007_role_victim AS
SELECT r.id AS victim_id, k.keep_id
FROM public.roles r
         JOIN _007_role_keep k ON k.name = r.name
WHERE r.id <> k.keep_id;

UPDATE public.users_roles ur
SET role_id = v.keep_id
FROM _007_role_victim v
WHERE ur.role_id = v.victim_id
  AND NOT EXISTS (SELECT 1
                  FROM public.users_roles x
                  WHERE x.user_id = ur.user_id
                    AND x.role_id = v.keep_id);

DELETE FROM public.users_roles ur
    USING _007_role_victim v
WHERE ur.role_id = v.victim_id;

DELETE FROM public.roles r
    USING _007_role_victim v
WHERE r.id = v.victim_id;

DROP TABLE IF EXISTS _007_role_victim;
DROP TABLE IF EXISTS _007_role_keep;

-- --- users_roles: NOT NULL + unique pair + indexes ---
ALTER TABLE public.users_roles
    ALTER COLUMN user_id SET NOT NULL,
    ALTER COLUMN role_id SET NOT NULL;

ALTER TABLE public.users_roles
    ADD CONSTRAINT users_roles_user_id_role_id_key UNIQUE (user_id, role_id);

CREATE INDEX IF NOT EXISTS users_roles_user_id_idx ON public.users_roles (user_id);
CREATE INDEX IF NOT EXISTS users_roles_role_id_idx ON public.users_roles (role_id);

-- --- roles: unique name ---
ALTER TABLE public.roles
    ADD CONSTRAINT roles_name_key UNIQUE (name);

-- users.email already UNIQUE (000001). refresh_sessions indexes from 000007.
