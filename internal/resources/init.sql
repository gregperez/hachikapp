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



-- new queries


-- Extensión necesaria para UUID en PostgreSQL
CREATE
EXTENSION IF NOT EXISTS "uuid-ossp";

-- Enumeraciones
CREATE TYPE status_process AS ENUM ('started', 'progress', 'completed', 'canceled');
CREATE TYPE status_stage AS ENUM ('pending', 'progress', 'completed', 'canceled');
CREATE TYPE status_task AS ENUM ('pending', 'progress', 'completed', 'rejected');
CREATE TYPE typeof_task AS ENUM ('file', 'aprobation', 'email', 'externalAction');
CREATE TYPE related_entity_type AS ENUM ('process', 'stage', 'task');

-- Tabla: processes
CREATE TABLE processes
(
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at  TIMESTAMP,
    created_at  TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by  VARCHAR,
    updated_by  VARCHAR,
    active      BOOLEAN          DEFAULT TRUE,
    messages    VARCHAR,
    name        VARCHAR        NOT NULL,
    description TEXT,
    status      status_process NOT NULL
);

-- Tabla: stages
CREATE TABLE stages
(
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at  TIMESTAMP,
    created_at  TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by  VARCHAR,
    updated_by  VARCHAR,
    active      BOOLEAN          DEFAULT TRUE,
    messages    VARCHAR,
    process_id  UUID REFERENCES processes (id) ON DELETE CASCADE,
    name        VARCHAR      NOT NULL,
    description TEXT,
    order_num   INT          NOT NULL,
    status      status_stage NOT NULL,
    start_date  TIMESTAMP,
    end_date    TIMESTAMP,
    limit_date  TIMESTAMP
);

-- Tabla: tasks
CREATE TABLE tasks
(
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at   TIMESTAMP,
    created_at   TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by   VARCHAR,
    updated_by   VARCHAR,
    active       BOOLEAN          DEFAULT TRUE,
    messages     VARCHAR,
    name         VARCHAR     NOT NULL,
    description  TEXT,
    stage_id     UUID REFERENCES stages (id) ON DELETE CASCADE,
    type_of_task typeof_task NOT NULL,
    status       status_task NOT NULL,
    limit_date   TIMESTAMP
);

-- Tabla: users
CREATE TABLE users
(
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at TIMESTAMP,
    created_at TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR,
    updated_by VARCHAR,
    active     BOOLEAN          DEFAULT TRUE,
    messages   VARCHAR,
    name       VARCHAR        NOT NULL,
    email      VARCHAR UNIQUE NOT NULL,
    password   VARCHAR        NOT NULL,
    is_admin   BOOLEAN          DEFAULT FALSE
);

-- Tabla: permissions
CREATE TABLE permissions
(
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at TIMESTAMP,
    created_at TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR,
    updated_by VARCHAR,
    active     BOOLEAN          DEFAULT TRUE,
    messages   VARCHAR,
    entity     VARCHAR NOT NULL,
    created     BOOLEAN          DEFAULT FALSE,
    read       BOOLEAN          DEFAULT FALSE,
    updated     BOOLEAN          DEFAULT FALSE,
    deleted     BOOLEAN          DEFAULT FALSE
);

-- Tabla: users_permissions
CREATE TABLE users_permissions
(
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at    TIMESTAMP,
    created_at    TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by    VARCHAR,
    updated_by    VARCHAR,
    active        BOOLEAN          DEFAULT TRUE,
    messages      VARCHAR,
    user_id       UUID REFERENCES users (id) ON DELETE CASCADE,
    permission_id UUID REFERENCES permissions (id) ON DELETE CASCADE
);

-- Tabla: task_managers
CREATE TABLE task_managers
(
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at TIMESTAMP,
    created_at TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR,
    updated_by VARCHAR,
    active     BOOLEAN          DEFAULT TRUE,
    messages   VARCHAR,
    task_id    UUID REFERENCES tasks (id) ON DELETE CASCADE,
    user_id    UUID REFERENCES users (id) ON DELETE CASCADE
);

-- Tabla: task_status_history
CREATE TABLE task_status_history
(
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at      TIMESTAMP,
    created_at      TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by      VARCHAR,
    updated_by      VARCHAR,
    active          BOOLEAN          DEFAULT TRUE,
    messages        VARCHAR,
    task_id         UUID REFERENCES tasks (id) ON DELETE CASCADE,
    user_id         UUID REFERENCES users (id) ON DELETE CASCADE,
    previous_status status_task,
    new_status      status_task,
    changed_at      TIMESTAMP
);

-- Tabla: clients
CREATE TABLE clients
(
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at TIMESTAMP,
    created_at TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR,
    updated_by VARCHAR,
    active     BOOLEAN          DEFAULT TRUE,
    messages   VARCHAR,
    first_name VARCHAR        NOT NULL,
    last_name  VARCHAR        NOT NULL,
    email      VARCHAR UNIQUE NOT NULL,
    phone      VARCHAR
);

-- Tabla: client_processes
CREATE TABLE client_processes
(
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at       TIMESTAMP,
    created_at       TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by       VARCHAR,
    updated_by       VARCHAR,
    active           BOOLEAN          DEFAULT TRUE,
    messages         VARCHAR,
    client_id        UUID REFERENCES clients (id) ON DELETE CASCADE,
    process_id       UUID REFERENCES processes (id) ON DELETE CASCADE,
    current_stage_id UUID REFERENCES stages (id),
    status           status_process NOT NULL,
    completed_at     TIMESTAMP
);

-- Tabla: client_stages
CREATE TABLE client_stages
(
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at        TIMESTAMP,
    created_at        TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by        VARCHAR,
    updated_by        VARCHAR,
    active            BOOLEAN          DEFAULT TRUE,
    messages          VARCHAR,
    client_process_id UUID REFERENCES client_processes (id) ON DELETE CASCADE,
    stage_id          UUID REFERENCES stages (id) ON DELETE CASCADE,
    status            status_stage NOT NULL,
    completed_at      TIMESTAMP
);

-- Tabla: client_tasks
CREATE TABLE client_tasks
(
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at      TIMESTAMP,
    created_at      TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by      VARCHAR,
    updated_by      VARCHAR,
    active          BOOLEAN          DEFAULT TRUE,
    messages        VARCHAR,
    client_stage_id UUID REFERENCES client_stages (id) ON DELETE CASCADE,
    task_id         UUID REFERENCES tasks (id) ON DELETE CASCADE,
    status          status_task NOT NULL,
    completed_at    TIMESTAMP,
    comments        TEXT
);

-- Tabla: history
CREATE TABLE history
(
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    updated_at      TIMESTAMP,
    created_at      TIMESTAMP        DEFAULT CURRENT_TIMESTAMP,
    created_by      VARCHAR,
    updated_by      VARCHAR,
    active          BOOLEAN          DEFAULT TRUE,
    messages        VARCHAR,
    entity_type     related_entity_type NOT NULL,
    entity_id       UUID                NOT NULL,
    client_id       UUID REFERENCES clients (id) ON DELETE CASCADE,
    previous_status VARCHAR,
    new_status      VARCHAR,
    comments        TEXT
);
