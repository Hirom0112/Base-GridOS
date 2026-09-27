BEGIN;

CREATE TABLE IF NOT EXISTS member_sites (
    site_id text PRIMARY KEY,
    member_id text NOT NULL,
    bound_at timestamptz NOT NULL,
    source text NOT NULL CHECK (source IN ('SIMULATED', 'MEMBER_AUTHORIZED')),
    provenance jsonb NOT NULL,
    CHECK (provenance->>'provenance' = source)
);

CREATE INDEX IF NOT EXISTS member_sites_member_id_idx ON member_sites (member_id);

ALTER TABLE travel_flex_windows
ADD COLUMN IF NOT EXISTS cancelled_at timestamptz;

ALTER TABLE travel_flex_windows
ADD COLUMN IF NOT EXISTS end_idempotency_key text;

ALTER TABLE travel_flex_windows
ADD COLUMN IF NOT EXISTS end_correlation_id text;

CREATE UNIQUE INDEX IF NOT EXISTS travel_flex_windows_end_key_unique
ON travel_flex_windows (end_idempotency_key);

ALTER TABLE travel_flex_windows
DROP CONSTRAINT IF EXISTS travel_flex_windows_cancelled_at_check;

ALTER TABLE travel_flex_windows
ADD CONSTRAINT travel_flex_windows_cancelled_at_check
CHECK (cancelled_at IS NULL OR (cancelled_at >= start_time AND cancelled_at < end_time));

COMMIT;
