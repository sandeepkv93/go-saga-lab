DROP INDEX IF EXISTS idx_outbox_events_lease_until;

ALTER TABLE outbox_events
    DROP COLUMN IF EXISTS lease_until,
    DROP COLUMN IF EXISTS lease_owner;
