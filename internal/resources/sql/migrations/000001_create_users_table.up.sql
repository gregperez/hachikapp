CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS public.users
(
    id         uuid      default uuid_generate_v4() not null
        primary key,
    updated_at timestamp,
    created_at timestamp default CURRENT_TIMESTAMP,
    created_by varchar,
    updated_by varchar,
    active     boolean   default true,
    messages   varchar,
    name       varchar                              not null,
    email      varchar                              not null
        unique,
    password   varchar                              not null,
    is_admin   boolean   default false
);

alter table public.users
    owner to postgres;