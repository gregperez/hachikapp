CREATE TABLE IF NOT EXISTS public.roles
(
    id          uuid      default gen_random_uuid() not null
        primary key,
    name        varchar(255)                        not null,
    routes      text[],
    permissions jsonb,
    updated_at  timestamp,
    created_at  timestamp default CURRENT_TIMESTAMP,
    created_by  varchar,
    updated_by  varchar,
    active      boolean   default true,
    messages    varchar
);

alter table roles
    owner to postgres;

