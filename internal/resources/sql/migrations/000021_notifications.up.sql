-- In-app notifications for pet owners and their email delivery queue
-- ("Ficha pública" part 2). Events from tag scans create notifications; each
-- notification may have one email delivery processed by the dispatcher.

CREATE TABLE public.notifications (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id uuid NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    kind varchar(24) NOT NULL,
    pet_id uuid NOT NULL REFERENCES public.pets (id) ON DELETE CASCADE,
    tag_id uuid NOT NULL REFERENCES public.pet_tags (id) ON DELETE CASCADE,
    scan_id uuid REFERENCES public.tag_scans (id) ON DELETE SET NULL,
    message_id uuid REFERENCES public.tag_messages (id) ON DELETE SET NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT notifications_kind_chk CHECK (kind IN ('tag_scanned', 'location_shared', 'finder_message', 'phone_revealed'))
);

CREATE INDEX notifications_user_idx ON public.notifications (user_id, created_at DESC, id DESC);
CREATE INDEX notifications_unread_idx ON public.notifications (user_id) WHERE read_at IS NULL;
CREATE INDEX notifications_scan_idx ON public.notifications (scan_id) WHERE scan_id IS NOT NULL;
CREATE INDEX notifications_created_idx ON public.notifications (created_at);

CREATE TABLE public.notification_deliveries (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    notification_id uuid NOT NULL REFERENCES public.notifications (id) ON DELETE CASCADE,
    channel varchar(16) NOT NULL DEFAULT 'email',
    status varchar(16) NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error varchar(300) NOT NULL DEFAULT '',
    sent_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT notification_deliveries_channel_chk CHECK (channel IN ('email')),
    CONSTRAINT notification_deliveries_status_chk CHECK (status IN ('pending', 'sent', 'failed')),
    CONSTRAINT notification_deliveries_attempts_chk CHECK (attempts >= 0)
);

CREATE UNIQUE INDEX notification_deliveries_notification_uq ON public.notification_deliveries (notification_id, channel);
CREATE INDEX notification_deliveries_due_idx ON public.notification_deliveries (next_attempt_at) WHERE status = 'pending';

CREATE TRIGGER notification_deliveries_entity_timestamps
    BEFORE INSERT OR UPDATE ON public.notification_deliveries
    FOR EACH ROW
EXECUTE FUNCTION public.set_entity_timestamps();
