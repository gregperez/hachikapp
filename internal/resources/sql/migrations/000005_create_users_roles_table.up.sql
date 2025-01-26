CREATE TABLE IF NOT EXISTS public.users_roles
(
    id         uuid      default uuid_generate_v4() not null
        primary key,
    updated_at timestamp,
    created_at timestamp default CURRENT_TIMESTAMP,
    created_by varchar,
    updated_by varchar,
    active     boolean   default true,
    messages   varchar,
    user_id    uuid
        references users
            on delete cascade,
    role_id    uuid
        references roles
            on delete cascade
);

alter table users_roles
    owner to postgres;

