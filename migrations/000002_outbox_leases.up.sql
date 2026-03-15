ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS lease_owner TEXT,
    ADD COLUMN IF NOT EXISTS lease_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_outbox_events_lease_until
    ON outbox_events (lease_until);
