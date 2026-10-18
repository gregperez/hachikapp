-- Reverse seeded admin identity (natural key). Safe if already revoked by 000006.
DELETE FROM public.users
WHERE email = 'admin@example.com';
