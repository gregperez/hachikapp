-- Pet records ("Ficha Mascota"). Each pet is managed by one account (owner_id);
-- the chip-registered owner and a second owner are stored as contact data.
-- Photo bytes live in pet_photos behind the PetPhotoPort so they can move to
-- object storage later; pets.photo_updated_at is the photo metadata.

CREATE TABLE public.pets (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id uuid NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    name varchar(60) NOT NULL,
    species varchar(16) NOT NULL,
    breed varchar(60) NOT NULL DEFAULT '',
    color varchar(60) NOT NULL DEFAULT '',
    sex varchar(16) NOT NULL DEFAULT 'unknown',
    birth_date date,
    chip_number varchar(15),
    weight_kg numeric(5, 2),
    size varchar(16) NOT NULL DEFAULT '',
    distinctive_features varchar(500) NOT NULL DEFAULT '',
    allergies text[] NOT NULL DEFAULT '{}',
    surgeries text[] NOT NULL DEFAULT '{}',
    special_conditions text[] NOT NULL DEFAULT '{}',
    temperament varchar(300) NOT NULL DEFAULT '',
    owner_full_name varchar(100) NOT NULL,
    owner_address varchar(200) NOT NULL DEFAULT '',
    owner_comuna varchar(60) NOT NULL DEFAULT '',
    owner_phone varchar(20) NOT NULL,
    owner_phone_alt varchar(20) NOT NULL DEFAULT '',
    second_owner_full_name varchar(100) NOT NULL DEFAULT '',
    second_owner_phone varchar(20) NOT NULL DEFAULT '',
    public_visibility jsonb NOT NULL DEFAULT '{}'::jsonb,
    photo_updated_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pets_name_chk CHECK (char_length(btrim(name)) BETWEEN 1 AND 60),
    CONSTRAINT pets_species_chk CHECK (species IN ('dog', 'cat', 'other')),
    CONSTRAINT pets_sex_chk CHECK (sex IN ('male', 'female', 'unknown')),
    CONSTRAINT pets_size_chk CHECK (size IN ('', 'small', 'medium', 'large', 'giant')),
    CONSTRAINT pets_chip_number_chk CHECK (chip_number IS NULL OR chip_number ~ '^[0-9]{9,15}$'),
    CONSTRAINT pets_weight_chk CHECK (weight_kg IS NULL OR (weight_kg > 0 AND weight_kg <= 150)),
    CONSTRAINT pets_owner_full_name_chk CHECK (char_length(btrim(owner_full_name)) BETWEEN 1 AND 100),
    CONSTRAINT pets_owner_phone_chk CHECK (char_length(btrim(owner_phone)) BETWEEN 8 AND 20),
    CONSTRAINT pets_public_visibility_chk CHECK (jsonb_typeof(public_visibility) = 'object'),
    CONSTRAINT pets_version_chk CHECK (version > 0)
);

-- A microchip identifies exactly one pet across the platform.
CREATE UNIQUE INDEX pets_chip_number_uq ON public.pets (chip_number) WHERE chip_number IS NOT NULL;
CREATE INDEX pets_owner_created_idx ON public.pets (owner_id, created_at DESC, id DESC);

CREATE TRIGGER pets_entity_timestamps
    BEFORE INSERT OR UPDATE ON public.pets
    FOR EACH ROW
EXECUTE FUNCTION public.set_entity_timestamps();

CREATE TABLE public.pet_photos (
    pet_id uuid PRIMARY KEY REFERENCES public.pets (id) ON DELETE CASCADE,
    content_type varchar(32) NOT NULL,
    size_bytes integer NOT NULL,
    data bytea NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pet_photos_content_type_chk CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    CONSTRAINT pet_photos_size_chk CHECK (size_bytes BETWEEN 1 AND 1048576 AND size_bytes = octet_length(data))
);
