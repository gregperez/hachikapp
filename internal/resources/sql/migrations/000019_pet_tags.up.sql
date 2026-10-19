-- Public pet tags ("Ficha pública"). Physical tags are generated in batches and
-- start unassigned; digital tags are created already active. A tag keeps
-- owner_id when it is released so only that account can re-link it.

CREATE TABLE public.tag_batches (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    label varchar(80) NOT NULL,
    quantity integer NOT NULL,
    created_by uuid REFERENCES public.users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tag_batches_label_chk CHECK (char_length(btrim(label)) BETWEEN 1 AND 80),
    CONSTRAINT tag_batches_quantity_chk CHECK (quantity BETWEEN 1 AND 1000)
);

CREATE INDEX tag_batches_created_idx ON public.tag_batches (created_at DESC, id DESC);

CREATE TABLE public.pet_tags (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    code char(10) NOT NULL,
    kind varchar(16) NOT NULL,
    batch_id uuid REFERENCES public.tag_batches (id) ON DELETE CASCADE,
    status varchar(16) NOT NULL DEFAULT 'unassigned',
    pet_id uuid REFERENCES public.pets (id) ON DELETE SET NULL,
    owner_id uuid REFERENCES public.users (id) ON DELETE SET NULL,
    activated_at timestamptz,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pet_tags_code_chk CHECK (code ~ '^[0-9A-HJKMNP-TV-Z]{10}$'),
    CONSTRAINT pet_tags_kind_chk CHECK (kind IN ('physical', 'digital')),
    CONSTRAINT pet_tags_status_chk CHECK (status IN ('unassigned', 'active', 'disabled')),
    CONSTRAINT pet_tags_batch_kind_chk CHECK (
        (kind = 'physical' AND batch_id IS NOT NULL) OR (kind = 'digital' AND batch_id IS NULL)
    ),
    CONSTRAINT pet_tags_active_chk CHECK (
        status <> 'active' OR (pet_id IS NOT NULL AND owner_id IS NOT NULL)
    ),
    CONSTRAINT pet_tags_unassigned_chk CHECK (status <> 'unassigned' OR pet_id IS NULL)
);

CREATE UNIQUE INDEX pet_tags_code_uq ON public.pet_tags (code);
CREATE INDEX pet_tags_batch_idx ON public.pet_tags (batch_id, created_at, id) WHERE batch_id IS NOT NULL;
CREATE INDEX pet_tags_pet_idx ON public.pet_tags (pet_id, created_at DESC, id DESC) WHERE pet_id IS NOT NULL;
CREATE INDEX pet_tags_active_pet_idx ON public.pet_tags (pet_id) WHERE status = 'active';

CREATE TRIGGER pet_tags_entity_timestamps
    BEFORE INSERT OR UPDATE ON public.pet_tags
    FOR EACH ROW
EXECUTE FUNCTION public.set_entity_timestamps();

-- Deleting a pet releases all its tags (active or disabled); owner_id is kept.
CREATE FUNCTION public.release_pet_tags()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE public.pet_tags
    SET status = 'unassigned',
        pet_id = NULL,
        activated_at = NULL,
        disabled_at = NULL
    WHERE pet_id = OLD.id;
    RETURN OLD;
END;
$$;

CREATE TRIGGER pets_release_tags
    BEFORE DELETE ON public.pets
    FOR EACH ROW
EXECUTE FUNCTION public.release_pet_tags();

CREATE TABLE public.tag_scans (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    tag_id uuid NOT NULL REFERENCES public.pet_tags (id) ON DELETE CASCADE,
    pet_id uuid NOT NULL REFERENCES public.pets (id) ON DELETE CASCADE,
    scanned_at timestamptz NOT NULL DEFAULT now(),
    visitor_hash char(64) NOT NULL,
    scan_token_hash char(64) NOT NULL,
    ip_prefix varchar(64) NOT NULL DEFAULT '',
    user_agent_family varchar(40) NOT NULL DEFAULT '',
    latitude numeric(9, 6),
    longitude numeric(9, 6),
    accuracy_m integer,
    location_at timestamptz,
    phone_revealed_at timestamptz,
    CONSTRAINT tag_scans_latitude_chk CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
    CONSTRAINT tag_scans_longitude_chk CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180),
    CONSTRAINT tag_scans_accuracy_chk CHECK (accuracy_m IS NULL OR accuracy_m >= 0)
);

CREATE INDEX tag_scans_pet_idx ON public.tag_scans (pet_id, scanned_at DESC, id DESC);
CREATE INDEX tag_scans_tag_idx ON public.tag_scans (tag_id, scanned_at DESC);
CREATE INDEX tag_scans_scanned_idx ON public.tag_scans (scanned_at);
CREATE UNIQUE INDEX tag_scans_token_uq ON public.tag_scans (scan_token_hash);

CREATE TABLE public.tag_messages (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    scan_id uuid NOT NULL REFERENCES public.tag_scans (id) ON DELETE CASCADE,
    tag_id uuid NOT NULL REFERENCES public.pet_tags (id) ON DELETE CASCADE,
    pet_id uuid NOT NULL REFERENCES public.pets (id) ON DELETE CASCADE,
    body varchar(500) NOT NULL,
    finder_name varchar(80) NOT NULL DEFAULT '',
    finder_phone varchar(20) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tag_messages_body_chk CHECK (char_length(btrim(body)) BETWEEN 1 AND 500)
);

CREATE INDEX tag_messages_scan_idx ON public.tag_messages (scan_id);
CREATE INDEX tag_messages_pet_idx ON public.tag_messages (pet_id, created_at DESC);
CREATE INDEX tag_messages_created_idx ON public.tag_messages (created_at);
