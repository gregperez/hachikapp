CREATE TABLE public.users
(
    id         serial  NOT NULL,
    email      varchar NOT NULL,
    "password" varchar NOT NULL,
    is_admin   boolean NULL,
    CONSTRAINT users_pk PRIMARY KEY (id)
);

CREATE TABLE public.permissions
(
    entity   varchar NULL,
    "create" boolean NULL,
    "read"   boolean NULL,
    "update" boolean NULL,
    "delete" boolean NULL,
    id       serial NOT NULL,
    CONSTRAINT permissions_pk PRIMARY KEY (id)
);

CREATE TABLE public.users_permissions
(
    id            serial NOT NULL,
    user_id       int8   NOT NULL,
    permission_id int8   NOT NULL,
    CONSTRAINT users_permissions_pk PRIMARY KEY (id),
    CONSTRAINT users_permissions_un UNIQUE (id, user_id, permission_id),
    CONSTRAINT users_permissions_fk FOREIGN KEY (user_id) REFERENCES public.users (id),
    CONSTRAINT users_permissions_fk_1 FOREIGN KEY (permission_id) REFERENCES public.permissions (id)
);

