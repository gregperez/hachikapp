-- A chip number is unique per account, not platform-wide. A global constraint
-- let anyone register someone else's chip first (locking the real owner out)
-- and revealed through the conflict whether a chip exists in the system.
-- Cross-account duplicates are left to a future transfer/dispute flow.
DROP INDEX IF EXISTS public.pets_chip_number_uq;

CREATE UNIQUE INDEX pets_owner_chip_number_uq
    ON public.pets (owner_id, chip_number)
    WHERE chip_number IS NOT NULL;
