-- Reverse default role seed by natural key (name).
DELETE FROM public.roles
WHERE name IN ('admin', 'user', 'guest');
