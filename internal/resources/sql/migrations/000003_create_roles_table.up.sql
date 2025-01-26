CREATE TABLE IF NOT EXISTS public.roles
(
    id          uuid default gen_random_uuid() not null
        primary key,
    name        varchar(255)                   not null,
    routes      text[],
    permissions jsonb
);

alter table roles
    owner to postgres;

