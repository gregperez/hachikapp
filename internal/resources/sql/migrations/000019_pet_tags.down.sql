DROP TABLE IF EXISTS public.tag_messages;
DROP TABLE IF EXISTS public.tag_scans;
DROP TRIGGER IF EXISTS pets_release_tags ON public.pets;
DROP FUNCTION IF EXISTS public.release_pet_tags();
DROP TABLE IF EXISTS public.pet_tags;
DROP TABLE IF EXISTS public.tag_batches;
