-- Restoring the platform-wide constraint fails if two accounts already share a
-- chip number; resolve those rows before rolling back.
DROP INDEX IF EXISTS public.pets_owner_chip_number_uq;

CREATE UNIQUE INDEX pets_chip_number_uq ON public.pets (chip_number) WHERE chip_number IS NOT NULL;
